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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter/logsv2"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/google/uuid"
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
	DISTRIBUTED_LOGS_TABLE                      = "distributed_logs"
	DISTRIBUTED_TAG_ATTRIBUTES                  = "distributed_tag_attributes"
	DISTRIBUTED_LOGS_TABLE_V2                   = "distributed_logs_v2"
	DISTRIBUTED_LOGS_RESOURCE_BUCKET_V2         = "distributed_logs_v2_resource_bucket"
	DISTRIBUTES_LOGS_RESOURCE_BUCKET_V2_SECONDS = 1800
)

type clickhouseLogsExporter struct {
	id              uuid.UUID
	db              clickhouse.Conn
	insertLogsSQL   string
	insertLogsSQLV2 string
	ksuid           ksuid.KSUID

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
	insertLogsSQLV2 := renderInsertLogsSQLV2(cfg)
	id := uuid.New()
	collector := usage.NewUsageCollector(
		id,
		client,
		usage.Options{
			ReportingInterval: usage.DefaultCollectionInterval,
		},
		"signoz_logs",
		UsageExporter,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating usage collector for logs : %v", err)
	}

	collector.Start()

	// view should be registered after exporter is initialized
	if err := view.Register(LogsCountView, LogsSizeView); err != nil {
		return nil, err
	}

	return &clickhouseLogsExporter{
		id:              id,
		db:              client,
		insertLogsSQL:   insertLogsSQL,
		insertLogsSQLV2: insertLogsSQLV2,
		logger:          logger,
		cfg:             cfg,
		ksuid:           ksuid.New(),
		usageCollector:  collector,
		wg:              new(sync.WaitGroup),
		closeChan:       make(chan struct{}),
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

func (e *clickhouseLogsExporter) getLogsTTLSeconds(ctx context.Context) (int, error) {
	type DBResponseTTL struct {
		EngineFull string `ch:"engine_full"`
	}

	var delTTL int = -1

	var dbResp []DBResponseTTL
	q := fmt.Sprintf("SELECT engine_full FROM system.tables WHERE name='%s' and database='%s'", tableName, databaseName)
	err := e.db.Select(ctx, &dbResp, q)
	if err != nil {
		return delTTL, err
	}
	if len(dbResp) == 0 {
		return delTTL, fmt.Errorf("ttl not found")
	}

	deleteTTLExp := regexp.MustCompile(`toIntervalSecond\(([0-9]*)\)`)

	m := deleteTTLExp.FindStringSubmatch(dbResp[0].EngineFull)
	if len(m) > 1 {
		seconds_int, err := strconv.Atoi(m[1])
		if err != nil {
			return delTTL, nil
		}
		delTTL = seconds_int
	}

	return delTTL, nil
}

func (e *clickhouseLogsExporter) removeOldLogs(ctx context.Context, ld plog.Logs) error {
	// get the TTL.
	ttL, err := e.getLogsTTLSeconds(ctx)
	if err != nil {
		return err
	}

	// if logs contains timestamp before acceptedDateTime, it will be rejected
	acceptedDateTime := time.Now().Add(-(time.Duration(ttL) * time.Second))

	removeLog := func(log plog.LogRecord) bool {
		t := log.Timestamp().AsTime()
		return t.Unix() < acceptedDateTime.Unix()
	}
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			logs.ScopeLogs().At(j).LogRecords().RemoveIf(removeLog)
		}
	}
	return nil
}

func (e *clickhouseLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	for i := 0; i <= 1; i++ {
		err := e.pushToClickhouse(ctx, ld)
		if err != nil {
			// StatementSend:code: 252, message: Too many partitions for single INSERT block
			// iterating twice since we want to try once after removing the old data
			if i == 1 || !strings.Contains(err.Error(), "StatementSend:code: 252") {
				// TODO(nitya): after returning it will be retried, ideally it should be pushed to DLQ
				return err
			}

			// drop logs older than TTL
			removeLogsError := e.removeOldLogs(ctx, ld)
			if removeLogsError != nil {
				return fmt.Errorf("error while dropping old logs %v, after error %v", removeLogsError, err)
			}
		} else {
			break
		}
	}

	return nil
}

func tsBucket(ts int64, bucketSize int64) int64 {
	return (int64(ts) / int64(bucketSize)) * int64(bucketSize)
}

func (e *clickhouseLogsExporter) pushToClickhouse(ctx context.Context, ld plog.Logs) error {
	resourcesSeen := map[int64]map[string]string{}

	var insertLogsStmtV2 driver.Batch
	var insertResourcesStmtV2 driver.Batch
	var statement driver.Batch
	var tagStatement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
		if tagStatement != nil {
			_ = tagStatement.Abort()
		}
		if insertLogsStmtV2 != nil {
			_ = insertLogsStmtV2.Abort()
		}
		if insertResourcesStmtV2 != nil {
			_ = insertResourcesStmtV2.Abort()
		}
	}()

	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
		start := time.Now()
		statement, err = e.db.PrepareBatch(ctx, e.insertLogsSQL, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareBatch:%w", err)
		}

		tagStatement, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, DISTRIBUTED_TAG_ATTRIBUTES), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareTagBatch:%w", err)
		}

		insertLogsStmtV2, err = e.db.PrepareBatch(ctx, e.insertLogsSQLV2, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareBatchV2:%w", err)
		}

		metrics := map[string]usage.Metric{}

		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resBytes, _ := json.Marshal(res.Attributes().AsRaw())

			resources := attributesToSlice(res.Attributes(), true)

			resourcesMap := attributesToMap(res.Attributes(), true)

			serializedRes, err := json.Marshal(res.Attributes().AsRaw())
			if err != nil {
				return fmt.Errorf("couldn't serialize log resource JSON: %w", err)
			}
			resourceJson := string(serializedRes)

			err = addAttrsToTagStatement(tagStatement, "resource", resources)
			if err != nil {
				return err
			}

			// remove after sometime
			resources = addTemporaryUnderscoreSupport(resources)

			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				scope := logs.ScopeLogs().At(j).Scope()
				scopeName := scope.Name()
				scopeVersion := scope.Version()

				scopeAttributes := attributesToSlice(scope.Attributes(), true)
				scopeMap := attributesToMap(scope.Attributes(), true)

				err := addAttrsToTagStatement(tagStatement, "scope", scopeAttributes)
				if err != nil {
					return err
				}

				rs := logs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

					// capturing the metrics
					tenant := usage.GetTenantNameFromResource(logs.Resource())
					attrBytes, _ := json.Marshal(r.Attributes().AsRaw())
					usage.AddMetric(metrics, tenant, 1, int64(len([]byte(r.Body().AsString()))+len(attrBytes)+len(resBytes)))

					// set observedTimestamp as the default timestamp if timestamp is empty.
					ts := uint64(r.Timestamp())
					ots := uint64(r.ObservedTimestamp())
					if ots == 0 {
						ots = uint64(time.Now().UnixNano())
					}
					if ts == 0 {
						ts = ots
					}

					lBucketStart := tsBucket(int64(ts/1000000000), DISTRIBUTES_LOGS_RESOURCE_BUCKET_V2_SECONDS)

					if _, exists := resourcesSeen[int64(lBucketStart)]; !exists {
						resourcesSeen[int64(lBucketStart)] = map[string]string{}
					}
					fp, exists := resourcesSeen[int64(lBucketStart)][resourceJson]
					if !exists {
						fp = logsv2.CalculateFingerprint(res.Attributes().AsRaw(), logsv2.ResourceHierarchy())
						resourcesSeen[int64(lBucketStart)][resourceJson] = fp
					}

					attributes := attributesToSlice(r.Attributes(), false)
					attrsMap := attributesToMap(r.Attributes(), false)

					err := addAttrsToTagStatement(tagStatement, "tag", attributes)
					if err != nil {
						return err
					}

					// remove after sometime
					attributes = addTemporaryUnderscoreSupport(attributes)

					err = insertLogsStmtV2.Append(
						uint64(lBucketStart),
						fp,
						ts,
						ots,
						e.ksuid.String(),
						utils.TraceIDToHexOrEmptyString(r.TraceID()),
						utils.SpanIDToHexOrEmptyString(r.SpanID()),
						uint32(r.Flags()),
						r.SeverityText(),
						uint8(r.SeverityNumber()),
						getStringifiedBody(r.Body()),
						attrsMap.StringData,
						attrsMap.NumberData,
						attrsMap.BoolData,
						resourcesMap.StringData,
						scopeName,
						scopeVersion,
						scopeMap.StringData,
					)
					if err != nil {
						return fmt.Errorf("StatementAppendLogsV2:%w", err)
					}

					// old table
					err = statement.Append(
						ts,
						ots,
						e.ksuid.String(),
						utils.TraceIDToHexOrEmptyString(r.TraceID()),
						utils.SpanIDToHexOrEmptyString(r.SpanID()),
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
						attributes.BoolKeys,
						attributes.BoolValues,
						scopeName,
						scopeVersion,
						scopeAttributes.StringKeys,
						scopeAttributes.StringValues,
					)
					if err != nil {
						return fmt.Errorf("StatementAppend:%w", err)
					}
					e.ksuid = e.ksuid.Next()
				}
			}
		}

		insertResourcesStmtV2, err = e.db.PrepareBatch(
			ctx,
			fmt.Sprintf("INSERT into %s.%s", databaseName, DISTRIBUTED_LOGS_RESOURCE_BUCKET_V2),
			driver.WithReleaseConnection(),
		)
		if err != nil {
			return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
		}

		for bucketTs, resources := range resourcesSeen {
			for resourceLabels, fingerprint := range resources {
				insertResourcesStmtV2.Append(
					resourceLabels,
					fingerprint,
					bucketTs,
				)
			}
		}

		dbWriteStart := time.Now()
		err = statement.Send()
		// insert to the new table
		errV2 := insertLogsStmtV2.Send()
		// insert into the resource bucket table
		errResource := insertResourcesStmtV2.Send()
		if err != nil {
			return fmt.Errorf("couldn't send batch insert resources statement:%w", err)
		}

		stats.RecordWithTags(ctx,
			[]tag.Mutator{
				tag.Upsert(exporterKey, component.DataTypeLogs.String()),
				tag.Upsert(tableKey, DISTRIBUTED_LOGS_TABLE),
			},
			writeLatencyMillis.M(int64(time.Since(dbWriteStart).Milliseconds())),
		)
		if err != nil {
			return fmt.Errorf("StatementSend:%w", err)
		}
		if errV2 != nil {
			return fmt.Errorf("StatementSendV2:%w", err)
		}
		if errResource != nil {
			return fmt.Errorf("ResourceStatementSendV2:%w", err)
		}

		duration := time.Since(start)
		e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
			zap.String("cost", duration.String()))

		for k, v := range metrics {
			stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, e.id.String())}, ExporterSigNozSentLogRecords.M(int64(v.Count)), ExporterSigNozSentLogRecordsBytes.M(int64(v.Size)))
		}

		// push tag attributes
		tagWriteStart := time.Now()
		err = tagStatement.Send()
		stats.RecordWithTags(ctx,
			[]tag.Mutator{
				tag.Upsert(exporterKey, component.DataTypeLogs.String()),
				tag.Upsert(tableKey, DISTRIBUTED_TAG_ATTRIBUTES),
			},
			writeLatencyMillis.M(int64(time.Since(tagWriteStart).Milliseconds())),
		)
		if err != nil {
			return err
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
	BoolKeys     []string
	BoolValues   []bool
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

func addAttrsToTagStatement(statement driver.Batch, tagType string, attrs attributesToSliceResponse) error {
	for i, v := range attrs.StringKeys {
		err := statement.Append(
			time.Now(),
			v,
			tagType,
			"string",
			attrs.StringValues[i],
			nil,
			nil,
		)
		if err != nil {
			return fmt.Errorf("could not append string attribute to batch, err: %s", err)
		}
	}
	for i, v := range attrs.IntKeys {
		err := statement.Append(
			time.Now(),
			v,
			tagType,
			"float64",
			nil,
			nil,
			attrs.IntValues[i],
		)
		if err != nil {
			return fmt.Errorf("could not append number attribute to batch, err: %s", err)
		}
	}
	for i, v := range attrs.FloatKeys {
		err := statement.Append(
			time.Now(),
			v,
			tagType,
			"float64",
			nil,
			nil,
			attrs.FloatValues[i],
		)
		if err != nil {
			return fmt.Errorf("could not append number attribute to batch, err: %s", err)
		}
	}
	for _, v := range attrs.BoolKeys {
		err := statement.Append(
			time.Now(),
			v,
			tagType,
			"bool",
			nil,
			nil,
			nil,
		)
		if err != nil {
			return fmt.Errorf("could not append bool attribute to batch, err: %s", err)
		}
	}
	return nil
}

type attributeMap struct {
	StringData map[string]string
	NumberData map[string]float64
	BoolData   map[string]bool
}

func attributesToMap(attributes pcommon.Map, forceStringValues bool) (response attributeMap) {
	response.BoolData = map[string]bool{}
	response.StringData = map[string]string{}
	response.NumberData = map[string]float64{}
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			// store everything as string
			response.StringData[k] = v.AsString()
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				response.NumberData[k] = float64(v.Int())
			case pcommon.ValueTypeDouble:
				response.NumberData[k] = v.Double()
			case pcommon.ValueTypeBool:
				response.BoolData[k] = v.Bool()
			default: // store it as string
				response.StringData[k] = v.AsString()
			}
		}

		return true
	})
	return response
}

func attributesToSlice(attributes pcommon.Map, forceStringValues bool) (response attributesToSliceResponse) {
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			// store everything as string
			response.StringKeys = append(response.StringKeys, k)
			response.StringValues = append(response.StringValues, v.AsString())
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				response.IntKeys = append(response.IntKeys, k)
				response.IntValues = append(response.IntValues, v.Int())
			case pcommon.ValueTypeDouble:
				response.FloatKeys = append(response.FloatKeys, k)
				response.FloatValues = append(response.FloatValues, v.Double())
			case pcommon.ValueTypeBool:
				response.BoolKeys = append(response.BoolKeys, k)
				response.BoolValues = append(response.BoolValues, v.Bool())
			default: // store it as string
				response.StringKeys = append(response.StringKeys, k)
				response.StringValues = append(response.StringValues, v.AsString())
			}
		}

		return true
	})
	return response
}

// remove this function after sometime.
func addTemporaryUnderscoreSupport(data attributesToSliceResponse) attributesToSliceResponse {
	for i, v := range data.BoolKeys {
		if strings.Contains(v, ".") {
			data.BoolKeys = append(data.BoolKeys, formatKey(v))
			data.BoolValues = append(data.BoolValues, data.BoolValues[i])
		}
	}

	for i, v := range data.StringKeys {
		if strings.Contains(v, ".") {
			data.StringKeys = append(data.StringKeys, formatKey(v))
			data.StringValues = append(data.StringValues, data.StringValues[i])
		}
	}
	for i, v := range data.IntKeys {
		if strings.Contains(v, ".") {
			data.IntKeys = append(data.IntKeys, formatKey(v))
			data.IntValues = append(data.IntValues, data.IntValues[i])
		}
	}
	for i, v := range data.FloatKeys {
		if strings.Contains(v, ".") {
			data.FloatKeys = append(data.FloatKeys, formatKey(v))
			data.FloatValues = append(data.FloatValues, data.FloatValues[i])
		}
	}

	return data
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
							attributes_float64_value,
							attributes_bool_key,
							attributes_bool_value,
							scope_name,
							scope_version,
							scope_string_key,
							scope_string_value
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
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								)`
)

const (
	// language=ClickHouse SQL
	insertLogsSQLTemplateV2 = `INSERT INTO %s.%s (
							ts_bucket_start,
							resource_fingerprint,
							timestamp,
							observed_timestamp,
							id,
							trace_id,
							span_id,
							trace_flags,
							severity_text,
							severity_number,
							body,
							attributes_string,
							attributes_number,
							attributes_bool,
							resources_string,
							scope_name,
							scope_version,
							scope_string
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
								?,
								?
								)`
)

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(logger *zap.Logger, cfg *Config) (clickhouse.Conn, error) {
	// use empty database to create database
	ctx := context.Background()
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	// setting maxIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	maxIdleConnections := cfg.QueueSettings.NumConsumers + 1
	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, databaseName, DISTRIBUTED_LOGS_TABLE)
}

func renderInsertLogsSQLV2(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplateV2, databaseName, DISTRIBUTED_LOGS_TABLE_V2)
}
