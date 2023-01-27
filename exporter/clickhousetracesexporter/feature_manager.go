package clickhousetracesexporter

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/featuregate"
)

const (
	DurationSortFeature  = "DURATION_SORT_FEATURE"
	TimestampSortFeature = "TIMESTAMP_SORT_FEATURE"
)

func init() {
	featuregate.GetRegistry().MustRegisterID(
		DurationSortFeature,
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("It controls use of materialized view which optimizes span duration based sorting for trace list at cost of extra disk usage"),
	)
	featuregate.GetRegistry().MustRegisterID(
		TimestampSortFeature,
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("It controls use of materialized view which optimizes span timestamp based sorting for trace list at cost of extra disk usage"),
	)
}

func initFeatures(db clickhouse.Conn, options *Options) error {
	if featuregate.GetRegistry().IsEnabled(DurationSortFeature) {
		err := enableDurationSortFeature(db, options)
		if err != nil {
			return err
		}
	} else {
		err := disableDurationSortFeature(db, options)
		if err != nil {
			return err
		}
	}
	if featuregate.GetRegistry().IsEnabled(TimestampSortFeature) {
		err := enableTimestampSortFeature(db, options)
		if err != nil {
			return err
		}
	} else {
		err := disableTimestampSortFeature(db, options)
		if err != nil {
			return err
		}
	}
	return nil
}

func enableDurationSortFeature(db clickhouse.Conn, options *Options) error {
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
		component LowCardinality(String) CODEC(ZSTD(1)),
		httpMethod LowCardinality(String) CODEC(ZSTD(1)),
		httpUrl LowCardinality(String) CODEC(ZSTD(1)), 
		httpCode LowCardinality(String) CODEC(ZSTD(1)), 
		httpRoute LowCardinality(String) CODEC(ZSTD(1)), 
		httpHost LowCardinality(String) CODEC(ZSTD(1)), 
		gRPCCode LowCardinality(String) CODEC(ZSTD(1)),
		gRPCMethod LowCardinality(String) CODEC(ZSTD(1)),
		hasError bool CODEC(T64, ZSTD(1)),
		tagMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
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
		INDEX idx_httpCode httpCode TYPE set(0) GRANULARITY 1,
		INDEX idx_hasError hasError TYPE set(2) GRANULARITY 1,
		INDEX idx_tagMapKeys mapKeys(tagMap) TYPE bloom_filter(0.01) GRANULARITY 64,
		INDEX idx_tagMapValues mapValues(tagMap) TYPE bloom_filter(0.01) GRANULARITY 64,
		INDEX idx_httpRoute httpRoute TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpUrl httpUrl TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpHost httpHost TYPE bloom_filter GRANULARITY 4,
		INDEX idx_httpMethod httpMethod TYPE bloom_filter GRANULARITY 4,
		INDEX idx_timestamp timestamp TYPE minmax GRANULARITY 1,
		INDEX idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
		INDEX idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1,
		) ENGINE MergeTree()
		PARTITION BY toDate(timestamp)
		ORDER BY (durationNano, timestamp)
		TTL toDateTime(timestamp) + INTERVAL 604800 SECOND DELETE
		SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1`, options.primary.TraceDatabase, options.primary.DurationSortTable, options.primary.Cluster))
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
		component,
		httpMethod,
		httpUrl,
		httpCode,
		httpRoute,
		httpHost,
		gRPCMethod,
		gRPCCode,
		hasError,
		tagMap,
		rpcSystem,
  		rpcService,
  		rpcMethod,
  		responseStatusCode,
		stringTagMap,
		numberTagMap,
		boolTagMap
		FROM %s.%s
		ORDER BY durationNano, timestamp`, options.primary.TraceDatabase, options.primary.DurationSortMVTable, options.primary.Cluster, options.primary.TraceDatabase, options.primary.DurationSortTable, options.primary.TraceDatabase, options.primary.IndexTable))
	if err != nil {
		return err
	}
	return nil
}

func disableDurationSortFeature(db clickhouse.Conn, options *Options) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s ON CLUSTER %s`, options.primary.TraceDatabase, options.primary.DurationSortTable, options.primary.Cluster))
	if err != nil {
		return err
	}
	err = db.Exec(context.Background(), fmt.Sprintf(`DROP VIEW IF EXISTS %s.%s ON CLUSTER %s`, options.primary.TraceDatabase, options.primary.DurationSortMVTable, options.primary.Cluster))
	if err != nil {
		return err
	}
	return nil
}

func enableTimestampSortFeature(db clickhouse.Conn, options *Options) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE %s.%s ON CLUSTER %s
	ADD PROJECTION IF NOT EXISTS timestampSort 
	( SELECT * ORDER BY timestamp )`, options.primary.TraceDatabase, options.primary.LocalIndexTable, options.primary.Cluster))
	if err != nil {
		return err
	}
	return nil
}

func disableTimestampSortFeature(db clickhouse.Conn, options *Options) error {
	err := db.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE %s.%s ON CLUSTER %s
	DROP PROJECTION IF EXISTS timestampSort`, options.primary.TraceDatabase, options.primary.LocalIndexTable, options.primary.Cluster))
	if err != nil {
		return err
	}
	return nil
}
