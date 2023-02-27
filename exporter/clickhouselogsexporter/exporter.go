// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouselogsexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/segmentio/ksuid"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	CLUSTER                = "cluster"
	DISTRIBUTED_LOGS_TABLE = "distributed_logs"
)

type clickhouseLogsExporter struct {
	db            clickhouse.Conn
	insertLogsSQL string
	ksuid         ksuid.KSUID

	logger *zap.Logger
	cfg    *Config

	usageCollector *usage.UsageCollector

	wg        *sync.WaitGroup
	closeChan chan struct{}
}

func newExporter(logger *zap.Logger, cfg *Config) (*clickhouseLogsExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClickhouseClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	insertLogsSQL := renderInsertLogsSQL(cfg)

	collector := usage.NewUsageCollector(
		client,
		usage.Options{
			ReportingInterval: usage.DefaultCollectionInterval,
		},
		"signoz_logs",
		UsageExporter,
	)
	if err != nil {
		log.Fatalf("Error creating usage collector for logs : %v", err)
	}

	collector.Start()

	// view should be registered after exporter is initialized
	if err := view.Register(LogsCountView, LogsSizeView); err != nil {
		return nil, err
	}

	return &clickhouseLogsExporter{
		db:             client,
		insertLogsSQL:  insertLogsSQL,
		logger:         logger,
		cfg:            cfg,
		ksuid:          ksuid.New(),
		usageCollector: collector,
		wg:             new(sync.WaitGroup),
		closeChan:      make(chan struct{}),
	}, nil
}

// Shutdown will shutdown the exporter.
func (e *clickhouseLogsExporter) Shutdown(_ context.Context) error {
	close(e.closeChan)
	e.wg.Wait()
	if e.usageCollector != nil {
		e.usageCollector.Stop()
	}
	if e.db != nil {
		err := e.db.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *clickhouseLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
		start := time.Now()
		statement, err := e.db.PrepareBatch(ctx, e.insertLogsSQL)
		if err != nil {
			return fmt.Errorf("PrepareBatch:%w", err)
		}
		defer func() {
			_ = statement.Abort()
		}()

		metrics := map[string]usage.Metric{}

		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resBytes, _ := json.Marshal(res.Attributes().AsRaw())

			resources := attributesToSlice(res.Attributes(), true)
			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				rs := logs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

					// capturing the metrics
					tenant := usage.GetTenantNameFromResource(logs.Resource())
					attrBytes, _ := json.Marshal(r.Attributes().AsRaw())
					usage.AddMetric(metrics, tenant, 1, int64(len([]byte(r.Body().AsString()))+len(attrBytes)+len(resBytes)))

					// set observedtimestamp as the default timestamp if timestamp is empty.
					ts := uint64(r.Timestamp())
					ots := uint64(r.ObservedTimestamp())
					if ots == 0 {
						ots = uint64(time.Now().UnixNano())
					}
					if ts == 0 {
						ts = ots
					}

					attributes := attributesToSlice(r.Attributes(), false)
					err = statement.Append(
						ts,
						ots,
						e.ksuid.String(),
						r.TraceID().HexString(),
						r.SpanID().HexString(),
						uint32(r.Flags()),
						r.SeverityText(),
						uint8(r.SeverityNumber()),
						getStringifiedBody(r.Body()),
						resources.StringKeys,
						resources.StringValues,
						attributes.StringKeys,
						attributes.StringValues,
						attributes.IntKeys,
						attributes.IntValues,
						attributes.FloatKeys,
						attributes.FloatValues,
					)
					if err != nil {
						return fmt.Errorf("StatementAppend:%w", err)
					}
					e.ksuid = e.ksuid.Next()
				}
			}
		}
		dbWriteStart := time.Now()
		err = statement.Send()
		stats.RecordWithTags(ctx,
			[]tag.Mutator{
				tag.Upsert(exporterKey, string(component.DataTypeLogs)),
				tag.Upsert(tableKey, DISTRIBUTED_LOGS_TABLE),
			},
			writeLatencyMillis.M(int64(time.Since(dbWriteStart).Milliseconds())),
		)
		if err != nil {
			return fmt.Errorf("StatementSend:%w", err)
		}
		duration := time.Since(start)
		e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
			zap.String("cost", duration.String()))

		for k, v := range metrics {
			stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k)}, ExporterSigNozSentLogRecords.M(int64(v.Count)), ExporterSigNozSentLogRecordsBytes.M(int64(v.Size)))
		}
		return err
	}
}

type attributesToSliceResponse struct {
	StringKeys   []string
	StringValues []string
	IntKeys      []string
	IntValues    []int64
	FloatKeys    []string
	FloatValues  []float64
}

func getStringifiedBody(body pcommon.Value) string {
	var strBody string
	switch body.Type() {
	case pcommon.ValueTypeBytes:
		strBody = string(body.Bytes().AsRaw())
	default:
		strBody = body.AsString()
	}
	return strBody
}

func attributesToSlice(attributes pcommon.Map, forceStringValues bool) (response attributesToSliceResponse) {
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			// store everything as string
			response.StringKeys = append(response.StringKeys, formatKey(k))
			response.StringValues = append(response.StringValues, v.AsString())
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				response.IntKeys = append(response.IntKeys, formatKey(k))
				response.IntValues = append(response.IntValues, v.Int())
			case pcommon.ValueTypeDouble:
				response.FloatKeys = append(response.FloatKeys, formatKey(k))
				response.FloatValues = append(response.FloatValues, v.Double())
			default: // store it as string
				response.StringKeys = append(response.StringKeys, formatKey(k))
				response.StringValues = append(response.StringValues, v.AsString())
			}
		}
		return true
	})
	return response
}

func formatKey(k string) string {
	return strings.ReplaceAll(k, ".", "_")
}

const (
	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s.%s (
							timestamp,
							observed_timestamp,
							id,
							trace_id,
							span_id,
							trace_flags,
							severity_text,
							severity_number,
							body,
							resources_string_key,
							resources_string_value,
							attributes_string_key, 
							attributes_string_value,
							attributes_int64_key,
							attributes_int64_value,
							attributes_float64_key,
							attributes_float64_value
							) VALUES (
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?
								)`
)

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(logger *zap.Logger, cfg *Config) (clickhouse.Conn, error) {
	// use empty database to create database
	ctx := context.Background()
	dsnURL, err := url.Parse(cfg.DSN)
	if err != nil {
		return nil, err
	}
	options := &clickhouse.Options{
		Addr: []string{dsnURL.Host},
	}
	if dsnURL.Query().Get("username") != "" {
		auth := clickhouse.Auth{
			Username: dsnURL.Query().Get("username"),
			Password: dsnURL.Query().Get("password"),
		}
		options.Auth = auth
	}
	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}

	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s ON CLUSTER %s;", databaseName, CLUSTER)
	err = db.Exec(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to create database, err: %s", err)
	}

	// drop schema migrations table if running in docker multi node cluster mode so that migrations are run on new nodes
	if cfg.DockerMultiNodeCluster {
		err = dropSchemaMigrationsTable(db)
		if err != nil {
			logger.Error("Error dropping schema_migrations table", zap.Error(err))
			return nil, err
		}
	}

	// do the migration here

	// get the migrations folder
	mgsFolder := os.Getenv("LOG_MIGRATIONS_FOLDER")
	if mgsFolder == "" {
		mgsFolder = migrationsFolder
	}

	logger.Info("Running migrations from path: ", zap.Any("test", mgsFolder))
	clickhouseUrl, err := buildClickhouseMigrateURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build Clickhouse migrate URL, error: %s", err)
	}
	m, err := migrate.New("file://"+mgsFolder, clickhouseUrl)
	if err != nil {
		return nil, fmt.Errorf("clickhouse Migrate failed to run, error: %s", err)
	}
	err = m.Up()
	if err != nil && !strings.HasSuffix(err.Error(), "no change") {
		return nil, fmt.Errorf("clickhouse Migrate failed to run, error: %s", err)
	}

	logger.Info("Clickhouse Migrate finished")
	return db, nil
}

func dropSchemaMigrationsTable(db clickhouse.Conn) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s ON CLUSTER %s;`,
		databaseName, "schema_migrations", CLUSTER))
	if err != nil {
		return fmt.Errorf("error dropping schema_migrations table: %v", err)
	}
	return nil
}

func buildClickhouseMigrateURL(cfg *Config) (string, error) {
	// return fmt.Sprintf("clickhouse://localhost:9000?database=default&x-multi-statement=true"), nil
	var clickhouseUrl string
	parsedURL, err := url.Parse(cfg.DSN)
	if err != nil {
		return "", err
	}
	host := parsedURL.Host
	if host == "" {
		return "", fmt.Errorf("unable to parse host")

	}
	paramMap, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		return "", err
	}
	username := paramMap["username"]
	password := paramMap["password"]

	if len(username) > 0 && len(password) > 0 {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", username[0], password[0], host, databaseName, CLUSTER)
	} else {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", host, databaseName, CLUSTER)
	}
	return clickhouseUrl, nil
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, databaseName, DISTRIBUTED_LOGS_TABLE)
}
