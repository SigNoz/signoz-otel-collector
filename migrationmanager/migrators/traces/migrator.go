package traces

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousetracesexporter"
	"github.com/SigNoz/signoz-otel-collector/migrationmanager/migrators/basemigrator"
)

const (
	name            = "traces"
	database        = "signoz_traces"
	migrationFolder = "migrationmanager/migrators/traces/migrations"
)

type TracesMigrator struct {
	*basemigrator.BaseMigrator
}

func (m *TracesMigrator) Migrate(ctx context.Context) error {
	err := m.BaseMigrator.Migrate(ctx, database, migrationFolder)
	if err != nil {
		return err
	}

	return m.initFeatures()
}

func (m *TracesMigrator) Name() string {
	return name
}

func (m *TracesMigrator) initFeatures() error {
	if m.Cfg.IsDurationSortFeatureDisabled {
		err := disableDurationSortFeature(m.DB, m.Cfg.ClusterName)
		if err != nil {
			return err
		}
	} else {
		err := enableDurationSortFeature(m.DB, m.Cfg.ClusterName, m.Cfg.ReplicationEnabled)
		if err != nil {
			return err
		}
	}
	if m.Cfg.IsTimestampSortFeatureDisabled {
		err := disableTimestampSortFeature(m.DB, m.Cfg.ClusterName)
		if err != nil {
			return err
		}
	} else {
		err := enableTimestampSortFeature(m.DB, m.Cfg.ClusterName)
		if err != nil {
			return err
		}
	}
	return nil
}

func enableDurationSortFeature(db clickhouse.Conn, cluster string, replicationEnabled bool) error {
	signozReplicated := ""
	if replicationEnabled {
		signozReplicated = "Replicated"
	}
	err := db.Exec(context.Background(), fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s ON CLUSTER %s( 
		timestamp DateTime64(9) CODEC(DoubleDelta, LZ4),
		traceID FixedString(32) CODEC(ZSTD(1)),
		spanID String CODEC(ZSTD(1)),
		parentSpanID String CODEC(ZSTD(1)),
		serviceName LowCardinality(String) CODEC(ZSTD(1)),
		name LowCardinality(String) CODEC(ZSTD(1)),
		kind Int8 CODEC(T64, ZSTD(1)),
		durationNano UInt64 CODEC(T64, ZSTD(1)),
		statusCode Int16 CODEC(T64, ZSTD(1)),
		httpMethod LowCardinality(String) CODEC(ZSTD(1)),
		httpUrl LowCardinality(String) CODEC(ZSTD(1)), 
		httpRoute LowCardinality(String) CODEC(ZSTD(1)), 
		httpHost LowCardinality(String) CODEC(ZSTD(1)), 
		hasError bool CODEC(T64, ZSTD(1)),
		rpcSystem LowCardinality(String) CODEC(ZSTD(1)),
		rpcService LowCardinality(String) CODEC(ZSTD(1)),
		rpcMethod LowCardinality(String) CODEC(ZSTD(1)),
		responseStatusCode LowCardinality(String) CODEC(ZSTD(1)),
		stringTagMap Map(String, String) CODEC(ZSTD(1)),
		numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
		boolTagMap Map(String, bool) CODEC(ZSTD(1)),
		INDEX idx_service serviceName TYPE bloom_filter GRANULARITY 4,
		INDEX idx_name name TYPE bloom_filter GRANULARITY 4,
		INDEX idx_kind kind TYPE minmax GRANULARITY 4,
		INDEX idx_duration durationNano TYPE minmax GRANULARITY 1,
		INDEX idx_hasError hasError TYPE set(2) GRANULARITY 1,
		INDEX idx_httpRoute httpRoute TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpUrl httpUrl TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpHost httpHost TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpMethod httpMethod TYPE bloom_filter GRANULARITY 4,
		INDEX idx_timestamp timestamp TYPE minmax GRANULARITY 1,
		INDEX idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
		INDEX idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1,
		) ENGINE %sMergeTree()
		PARTITION BY toDate(timestamp)
		ORDER BY (durationNano, timestamp)
		TTL toDateTime(timestamp) + INTERVAL 1296000 SECOND DELETE
		SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultDurationSortTable, cluster, signozReplicated))
	if err != nil {
		return err
	}
	err = db.Exec(context.Background(), fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s ON CLUSTER %s
		TO %s.%s
		AS SELECT
		timestamp,
		traceID,
		spanID,
		parentSpanID,
		serviceName,
		name,
		kind,
		durationNano,
		statusCode,
		httpMethod,
		httpUrl,
		httpRoute,
		httpHost,
		hasError,
		rpcSystem,
  		rpcService,
  		rpcMethod,
  		responseStatusCode,
		stringTagMap,
		numberTagMap,
		boolTagMap
		FROM %s.%s
		ORDER BY durationNano, timestamp`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultDurationSortMVTable, cluster, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultDurationSortTable, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultIndexTable))
	if err != nil {
		return err
	}
	return nil
}

func disableDurationSortFeature(db clickhouse.Conn, cluster string) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s ON CLUSTER %s`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultDurationSortTable, cluster))
	if err != nil {
		return err
	}
	err = db.Exec(context.Background(), fmt.Sprintf(`DROP VIEW IF EXISTS %s.%s ON CLUSTER %s`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.DefaultDurationSortMVTable, cluster))
	if err != nil {
		return err
	}
	return nil
}

func enableTimestampSortFeature(db clickhouse.Conn, cluster string) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE %s.%s ON CLUSTER %s
	ADD PROJECTION IF NOT EXISTS timestampSort 
	( SELECT * ORDER BY timestamp )`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.LocalIndexTable, cluster))
	if err != nil {
		return err
	}
	return nil
}

func disableTimestampSortFeature(db clickhouse.Conn, cluster string) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE %s.%s ON CLUSTER %s
	DROP PROJECTION IF EXISTS timestampSort`, clickhousetracesexporter.DefaultTraceDatabase, clickhousetracesexporter.LocalIndexTable, cluster))
	if err != nil {
		return err
	}
	return nil
}
