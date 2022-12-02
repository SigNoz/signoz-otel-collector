// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Factory implements storage.Factory for Clickhouse backend.
type Factory struct {
	logger     *zap.Logger
	Options    *Options
	db         clickhouse.Conn
	archive    clickhouse.Conn
	datasource string
	makeWriter writerMaker
}

// Writer writes spans to storage.
type Writer interface {
	WriteSpan(span *Span) error
}

type writerMaker func(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding, delay time.Duration, size int) (Writer, error)

// NewFactory creates a new Factory.
func ClickHouseNewFactory(migrations string, datasource string) *Factory {
	return &Factory{
		Options: NewOptions(migrations, datasource, primaryNamespace, archiveNamespace),
		// makeReader: func(db *clickhouse.Conn, operationsTable, indexTable, spansTable string) (spanstore.Reader, error) {
		// 	return store.NewTraceReader(db, operationsTable, indexTable, spansTable), nil
		// },
		makeWriter: func(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding, delay time.Duration, size int) (Writer, error) {
			return NewSpanWriter(logger, db, traceDatabase, spansTable, indexTable, errorTable, encoding, delay, size), nil
		},
	}
}

// Initialize implements storage.Factory
func (f *Factory) Initialize(logger *zap.Logger) error {
	f.logger = logger

	db, err := f.connect(f.Options.getPrimary())
	if err != nil {
		return fmt.Errorf("error connecting to primary db: %v", err)
	}

	f.db = db

	archiveConfig := f.Options.others[archiveNamespace]
	if archiveConfig.Enabled {
		archive, err := f.connect(archiveConfig)
		if err != nil {
			return fmt.Errorf("error connecting to archive db: %v", err)
		}

		f.archive = archive
	}

	err = patchGroupByParenInMV(db, f)
	if err != nil {
		return err
	}

	f.logger.Info("Running migrations from path: ", zap.Any("test", f.Options.primary.Migrations))
	clickhouseUrl, err := buildClickhouseMigrateURL(f.Options.primary.Datasource, f.Options.primary.Cluster)
	if err != nil {
		return fmt.Errorf("Failed to build Clickhouse migrate URL, error: %s", err)
	}
	m, err := migrate.New(
		"file://"+f.Options.primary.Migrations,
		clickhouseUrl)
	if err != nil {
		return fmt.Errorf("Clickhouse Migrate failed to run, error: %s", err)
	}
	err = m.Up()
	f.logger.Info("Clickhouse Migrate finished", zap.Error(err))
	return nil
}

func patchGroupByParenInMV(db clickhouse.Conn, f *Factory) error {

	// check if views already exist, if not, skip as patch is not required for fresh install
	for _, table := range []string{f.Options.getPrimary().DependencyGraphDbMV, f.Options.getPrimary().DependencyGraphServiceMV, f.Options.getPrimary().DependencyGraphMessagingMV} {
		var exists uint8
		err := db.QueryRow(context.Background(), fmt.Sprintf("EXISTS VIEW %s.%s", f.Options.getPrimary().TraceDatabase, table)).Scan(&exists)
		if err != nil {
			return err
		}
		if exists == 0 {
			f.logger.Info("View does not exist, skipping patch", zap.String("table", table))
			return nil
		}
	}
	f.logger.Info("Patching views")
	// drop views
	for _, table := range []string{f.Options.getPrimary().DependencyGraphDbMV, f.Options.getPrimary().DependencyGraphServiceMV, f.Options.getPrimary().DependencyGraphMessagingMV} {
		err := db.Exec(context.Background(), fmt.Sprintf("DROP VIEW IF EXISTS %s.%s ON CLUSTER %s", f.Options.getPrimary().TraceDatabase, table, f.Options.getPrimary().Cluster))
		if err != nil {
			f.logger.Error(fmt.Sprintf("Error dropping %s view", table), zap.Error(err))
			return fmt.Errorf("error dropping %s view: %v", table, err)
		}
	}

	// create views with patched group by
	err := db.Exec(context.Background(), fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s ON CLUSTER %s
		TO %s.%s AS
		SELECT
			A.serviceName as src,
			B.serviceName as dest,
			quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.durationNano)) as duration_quantiles_state,
			countIf(B.statusCode=2) as error_count,
			count(*) as total_count,
			toStartOfMinute(B.timestamp) as timestamp
		FROM %s.%s AS A, %s.%s AS B
		WHERE (A.serviceName != B.serviceName) AND (A.spanID = B.parentSpanID)
		GROUP BY timestamp, src, dest;`, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphServiceMV,
		f.Options.getPrimary().Cluster, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphTable,
		f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().LocalIndexTable, f.Options.getPrimary().TraceDatabase,
		f.Options.getPrimary().LocalIndexTable))
	if err != nil {
		f.logger.Error("Error creating "+f.Options.getPrimary().DependencyGraphServiceMV, zap.Error(err))
		return fmt.Errorf("error creating %s: %v", f.Options.getPrimary().DependencyGraphServiceMV, err)
	}
	err = db.Exec(context.Background(), fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s ON CLUSTER %s
		TO %s.%s AS
		SELECT
			serviceName as src,
			tagMap['db.system'] as dest,
			quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
			countIf(statusCode=2) as error_count,
			count(*) as total_count,
			toStartOfMinute(timestamp) as timestamp
		FROM %s.%s
		WHERE dest != '' and kind != 2
		GROUP BY timestamp, src, dest;`, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphDbMV,
		f.Options.getPrimary().Cluster, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphTable,
		f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().LocalIndexTable))
	if err != nil {
		f.logger.Error("Error creating "+f.Options.getPrimary().DependencyGraphDbMV, zap.Error(err))
		return fmt.Errorf("error creating %s: %v", f.Options.getPrimary().DependencyGraphDbMV, err)
	}
	err = db.Exec(context.Background(), fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s ON CLUSTER %s
		TO %s.%s AS
		SELECT
			serviceName as src,
			tagMap['messaging.system'] as dest,
			quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
			countIf(statusCode=2) as error_count,
			count(*) as total_count,
			toStartOfMinute(timestamp) as timestamp
		FROM %s.%s
		WHERE dest != '' and kind != 2
		GROUP BY timestamp, src, dest;`, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphMessagingMV,
		f.Options.getPrimary().Cluster, f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().DependencyGraphTable,
		f.Options.getPrimary().TraceDatabase, f.Options.getPrimary().LocalIndexTable))
	if err != nil {
		f.logger.Error("Error creating "+f.Options.getPrimary().DependencyGraphMessagingMV, zap.Error(err))
		return fmt.Errorf("error creating %s: %v", f.Options.getPrimary().DependencyGraphMessagingMV, err)
	}

	return nil
}

func buildClickhouseMigrateURL(datasource string, cluster string) (string, error) {
	// return fmt.Sprintf("clickhouse://localhost:9000?database=default&x-multi-statement=true"), nil
	var clickhouseUrl string
	database := "signoz_traces"
	parsedURL, err := url.Parse(datasource)
	if err != nil {
		return "", err
	}
	host := parsedURL.Host
	if host == "" {
		return "", fmt.Errorf("Unable to parse host")

	}
	paramMap, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		return "", err
	}
	username := paramMap["username"]
	password := paramMap["password"]

	if len(username) > 0 && len(password) > 0 {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", username[0], password[0], host, database, cluster)
	} else {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", host, database, cluster)
	}
	return clickhouseUrl, nil
}

func (f *Factory) connect(cfg *namespaceConfig) (clickhouse.Conn, error) {
	if cfg.Encoding != EncodingJSON && cfg.Encoding != EncodingProto {
		return nil, fmt.Errorf("unknown encoding %q, supported: %q, %q", cfg.Encoding, EncodingJSON, EncodingProto)
	}

	return cfg.Connector(cfg)
}

// AddFlags implements plugin.Configurable
func (f *Factory) AddFlags(flagSet *flag.FlagSet) {
	f.Options.AddFlags(flagSet)
}

// InitFromViper implements plugin.Configurable
func (f *Factory) InitFromViper(v *viper.Viper) {
	f.Options.InitFromViper(v)
}

// CreateSpanWriter implements storage.Factory
func (f *Factory) CreateSpanWriter() (Writer, error) {
	cfg := f.Options.getPrimary()
	return f.makeWriter(f.logger, f.db, cfg.TraceDatabase, cfg.SpansTable, cfg.IndexTable, cfg.ErrorTable, cfg.Encoding, cfg.WriteBatchDelay, cfg.WriteBatchSize)
}

// CreateArchiveSpanWriter implements storage.ArchiveFactory
func (f *Factory) CreateArchiveSpanWriter() (Writer, error) {
	if f.archive == nil {
		return nil, nil
	}
	cfg := f.Options.others[archiveNamespace]
	return f.makeWriter(f.logger, f.archive, "", cfg.TraceDatabase, cfg.SpansTable, cfg.ErrorTable, cfg.Encoding, cfg.WriteBatchDelay, cfg.WriteBatchSize)
}

// Close Implements io.Closer and closes the underlying storage
func (f *Factory) Close() error {
	if f.db != nil {
		err := f.db.Close()
		if err != nil {
			return err
		}

		f.db = nil
	}

	if f.archive != nil {
		err := f.archive.Close()
		if err != nil {
			return err
		}

		f.archive = nil
	}

	return nil
}
