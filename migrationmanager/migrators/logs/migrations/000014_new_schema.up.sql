CREATE TABLE IF NOT EXISTS  signoz_logs.logs_v2_resource ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `labels` String CODEC(ZSTD(5)),
    `fingerprint` String CODEC(ZSTD(1)),
    `seen_at_ts_bucket_start` Int64 CODEC(Delta(8), ZSTD(1)),
    INDEX idx_labels lower(labels) TYPE ngrambf_v1(4, 1024, 3, 0) GRANULARITY 1
)
ENGINE = {{.SIGNOZ_REPLICATED}}ReplacingMergeTree
PARTITION BY toDate(seen_at_ts_bucket_start / 1000)
ORDER BY (labels, fingerprint, seen_at_ts_bucket_start)
TTL toDateTime(seen_at_ts_bucket_start) + INTERVAL 1296000 SECOND + INTERVAL 1800 SECOND DELETE
SETTINGS ttl_only_drop_parts = 1, index_granularity = 8192;


CREATE TABLE IF NOT EXISTS  signoz_logs.distributed_logs_v2_resource ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `labels` String CODEC(ZSTD(5)),
    `fingerprint` String CODEC(ZSTD(1)),
    `seen_at_ts_bucket_start` Int64 CODEC(Delta(8), ZSTD(1))
)
ENGINE = Distributed({{.SIGNOZ_CLUSTER}}, 'signoz_logs', 'logs_v2_resource', cityHash64(labels, fingerprint));


CREATE TABLE IF NOT EXISTS  signoz_logs.logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `ts_bucket_start` UInt64 CODEC(DoubleDelta, LZ4),
    `resource_fingerprint` String CODEC(ZSTD(1)),
    `timestamp` UInt64 CODEC(DoubleDelta, LZ4),
    `observed_timestamp` UInt64 CODEC(DoubleDelta, LZ4),
    `id` String CODEC(ZSTD(1)),
    `trace_id` String CODEC(ZSTD(1)),
    `span_id` String CODEC(ZSTD(1)),
    `trace_flags` UInt32,
    `severity_text` LowCardinality(String) CODEC(ZSTD(1)),
    `severity_number` UInt8,
    `body` String CODEC(ZSTD(2)),
    `attributes_string` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    `attributes_number` Map(LowCardinality(String), Float64) CODEC(ZSTD(1)),
    `attributes_bool` Map(LowCardinality(String), Bool) CODEC(ZSTD(1)),
    `resources_string` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    `scope_name` String CODEC(ZSTD(1)),
    `scope_version` String CODEC(ZSTD(1)),
    `scope_string` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    INDEX body_idx lower(body) TYPE ngrambf_v1(4, 60000, 5, 0) GRANULARITY 1,
    INDEX id_minmax id TYPE minmax GRANULARITY 1,
    INDEX severity_number_idx severity_number TYPE set(25) GRANULARITY 4,
    INDEX severity_text_idx severity_text TYPE set(25) GRANULARITY 4,
    INDEX trace_flags_idx trace_flags TYPE bloom_filter GRANULARITY 4,
    INDEX scope_name_idx scope_name TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4,
    INDEX attributes_string_idx_key mapKeys(attributes_string) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1,
    INDEX attributes_string_idx_val mapValues(attributes_string) TYPE ngrambf_v1(4, 5000, 2, 0) GRANULARITY 1,
    INDEX attributes_int64_idx_key mapKeys(attributes_number) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1,
    INDEX attributes_int64_idx_val mapValues(attributes_number) TYPE bloom_filter GRANULARITY 1,
    INDEX attributes_bool_idx_key mapKeys(attributes_bool) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1
)
ENGINE = {{.SIGNOZ_REPLICATED}}MergeTree
PARTITION BY toDate(timestamp / 1000000000)
ORDER BY (ts_bucket_start, resource_fingerprint, severity_text, timestamp, id)
TTL toDateTime(timestamp / 1000000000) + INTERVAL 1296000 SECOND DELETE
SETTINGS ttl_only_drop_parts = 1, index_granularity = 8192;



CREATE TABLE IF NOT EXISTS  signoz_logs.distributed_logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `ts_bucket_start` UInt64 CODEC(DoubleDelta, LZ4),
    `resource_fingerprint` String CODEC(ZSTD(1)),
    `timestamp` UInt64 CODEC(DoubleDelta, LZ4),
    `observed_timestamp` UInt64 CODEC(DoubleDelta, LZ4),
    `id` String CODEC(ZSTD(1)),
    `trace_id` String CODEC(ZSTD(1)),
    `span_id` String CODEC(ZSTD(1)),
    `trace_flags` UInt32,
    `severity_text` LowCardinality(String) CODEC(ZSTD(1)),
    `severity_number` UInt8,
    `body` String CODEC(ZSTD(2)),
    `attributes_string` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    `attributes_number` Map(LowCardinality(String), Float64) CODEC(ZSTD(1)),
    `attributes_bool` Map(LowCardinality(String), Bool) CODEC(ZSTD(1)),
    `resources_string` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    `scope_name` String CODEC(ZSTD(1)),
    `scope_version` String CODEC(ZSTD(1)),
    `scope_string` Map(LowCardinality(String), String) CODEC(ZSTD(1))
)
ENGINE = Distributed({{.SIGNOZ_CLUSTER}}, 'signoz_logs', 'logs_v2', cityHash64(id));

-- remove the old mv
DROP TABLE IF EXISTS signoz_logs.resource_keys_string_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_float64_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_int64_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_string_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_bool_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};


CREATE MATERIALIZED VIEW IF NOT EXISTS  attribute_keys_string_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(mapKeys(attributes_string)) as name, 'String' datatype
FROM signoz_logs.logs_v2
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  attribute_keys_float64_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(mapKeys(attributes_number)) as name, 'Float64' datatype
FROM signoz_logs.logs_v2
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  signoz_logs.attribute_keys_bool_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(mapKeys(attributes_bool)) as name, 'Bool' datatype
FROM signoz_logs.logs_v2
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  resource_keys_string_final_mv  ON CLUSTER  {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_resource_keys AS
SELECT
distinct arrayJoin(mapKeys(resources_string)) as name, 'String' datatype
FROM signoz_logs.logs_v2
ORDER BY  name;