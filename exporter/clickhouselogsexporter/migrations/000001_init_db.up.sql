-- https://altinity.com/blog/2019/7/new-encodings-to-improve-clickhouse
CREATE TABLE IF NOT EXISTS logs ON CLUSTER signoz (
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
) ENGINE ReplicatedMergeTree()
PARTITION BY toDate(timestamp / 1000000000)
ORDER BY (timestamp, id);


CREATE TABLE distributed_logs ON CLUSTER signoz AS logs
ENGINE = Distributed("signoz", currentDatabase(), logs, cityHash64(id));