-- https://altinity.com/blog/2019/7/new-encodings-to-improve-clickhouse
CREATE TABLE IF NOT EXISTS signoz_logs.logs ON CLUSTER signoz (
	timestamp UInt64 CODEC(DoubleDelta, LZ4),
	observed_timestamp UInt64 CODEC(DoubleDelta, LZ4),
	id String CODEC(ZSTD(1)),
	trace_id String CODEC(ZSTD(1)),
	span_id String CODEC(ZSTD(1)),
	trace_flags UInt32,
	severity_text LowCardinality(String) CODEC(ZSTD(1)),
	severity_number UInt8,
	body String CODEC(ZSTD(2)),
	resources_string_key Array(String) CODEC(ZSTD(1)),
	resources_string_value Array(String) CODEC(ZSTD(1)),
	attributes_string_key Array(String) CODEC(ZSTD(1)),
	attributes_string_value Array(String) CODEC(ZSTD(1)),
	attributes_int64_key Array(String) CODEC(ZSTD(1)),
	attributes_int64_value Array(Int64) CODEC(ZSTD(1)),
	attributes_float64_key Array(String) CODEC(ZSTD(1)),
	attributes_float64_value Array(Float64) CODEC(ZSTD(1)),
	INDEX body_idx body TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4
) ENGINE ReplicatedMergeTree('/clickhouse/tables/{cluster}/{shard}/signoz_logs/logs', '{replica}')
PARTITION BY toDate(timestamp / 1000000000)
ORDER BY (timestamp, id);


CREATE TABLE IF NOT EXISTS signoz_logs.distributed_logs  ON CLUSTER signoz AS signoz_logs.logs
ENGINE = Distributed("signoz", "signoz_logs", logs, cityHash64(id));

CREATE TABLE IF NOT EXISTS signoz_logs.schema_migrations ON CLUSTER signoz (
  version Int64,
  dirty UInt8,
  sequence UInt64
) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{shard}/signoz_logs/schema_migrations', '{replica}')
ORDER BY version;

CREATE TABLE IF NOT EXISTS signoz_logs.distributed_schema_migrations  ON CLUSTER signoz AS signoz_logs.schema_migrations
ENGINE = Distributed("signoz", "signoz_logs", schema_migrations, rand());