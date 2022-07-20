CREATE TABLE IF NOT EXISTS logs (
	timestamp UInt64 CODEC(Delta, ZSTD(1)),
	observed_timestamp UInt64 CODEC(Delta, ZSTD(1)),
	id String CODEC(ZSTD(1)),
	trace_id String CODEC(ZSTD(1)),
	span_id String CODEC(ZSTD(1)),
	trace_flags UInt32,
	severity_text LowCardinality(String) CODEC(ZSTD(1)),
	severity_number Int32,
	body String CODEC(ZSTD(1)),
	resources_string_key Array(String) CODEC(ZSTD(1)),
	resources_string_value Array(String) CODEC(ZSTD(1)),
	attributes_string_key Array(String) CODEC(ZSTD(1)),
	attributes_string_value Array(String) CODEC(ZSTD(1)),
	attributes_int64_key Array(String) CODEC(ZSTD(1)),
	attributes_int64_value Array(Int64) CODEC(ZSTD(1)),
	attributes_float64_key Array(String) CODEC(ZSTD(1)),
	attributes_float64_value Array(Float64) CODEC(ZSTD(1))
) ENGINE MergeTree()
PARTITION BY toDate(timestamp / 1000000000)
ORDER BY (timestamp, id);


CREATE TABLE IF NOT EXISTS logs_atrribute_keys (
name String,
datatype String
)ENGINE = AggregatingMergeTree
ORDER BY (name);

CREATE TABLE IF NOT EXISTS logs_resource_keys (
name String,
datatype String
)ENGINE = AggregatingMergeTree
ORDER BY (name);

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_string_final_mv TO logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_string_key) as name, 'String' datatype
FROM logs
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_int64_final_mv TO logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_int64_key) as name, 'Int64' datatype
FROM logs
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_float64_final_mv TO logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_float64_key) as name, 'Float64' datatype
FROM logs
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  resource_keys_string_final_mv TO logs_resource_keys AS
SELECT
distinct arrayJoin(resources_string_key) as name, 'String' datatype
FROM logs
ORDER BY  name;