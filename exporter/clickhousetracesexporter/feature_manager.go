package clickhousetracesexporter

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/featuregate"
)

const (
	DurationSortFeature  = "DURATION_SORT_FEATURE"
	TimestampSortFeature = "TIMESTAMP_SORT_FEATURE"
)

func init() {
	featuregate.GetRegistry().Register(featuregate.Gate{
		ID:          DurationSortFeature,
		Description: "A brief description of what the gate controls",
		Enabled:     false,
	})
	featuregate.GetRegistry().Register(featuregate.Gate{
		ID:          TimestampSortFeature,
		Description: "A brief description of what the gate controls",
		Enabled:     false,
	})
}

func initFeatures(db clickhouse.Conn) error {
	if featuregate.GetRegistry().IsEnabled(DurationSortFeature) {
		err := enableDurationSortFeature(db)
		if err != nil {
			return err
		}
	} else {
		err := disableDurationSortFeature(db)
		if err != nil {
			return err
		}
	}
	// if featuregate.GetRegistry().IsEnabled(TimestampSortFeature) {
	// 	err := enableTimestampSortFeature(db)
	// 	if err != nil {
	// 		return err
	// 	}
	// } else {
	// 	err := disableTimestampSortFeature(db)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

func enableDurationSortFeature(db clickhouse.Conn) error {
	err := db.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS signoz_traces.durationSort ( 
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
		INDEX idx_timestamp timestamp TYPE minmax GRANULARITY 1
		) ENGINE MergeTree()
		PARTITION BY toDate(timestamp)
		ORDER BY (durationNano, timestamp)
		SETTINGS index_granularity = 8192`)
	if err != nil {
		return err
	}
	err = db.Exec(context.Background(), `CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.durationSortMV
		TO signoz_traces.durationSort
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
		tagMap
		FROM signoz_traces.signoz_index_v2
		ORDER BY durationNano, timestamp`)
	if err != nil {
		return err
	}
	return nil
}

func disableDurationSortFeature(db clickhouse.Conn) error {
	err := db.Exec(context.Background(), "DROP TABLE IF EXISTS signoz_traces.durationSort")
	if err != nil {
		return err
	}
	err = db.Exec(context.Background(), "DROP VIEW IF EXISTS signoz_traces.durationSortMV")
	if err != nil {
		return err
	}
	return nil
}
