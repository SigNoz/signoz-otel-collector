CREATE TABLE IF NOT EXISTS signoz_metrics.metric_labels ON CLUSTER {{.SIGNOZ_CLUSTER}} (
    unix_milli Int64 CODEC(ZSTD(1)),
    name LowCardinality(String) CODEC(ZSTD(1)),
    type Enum('tag', 'resource') CODEC(ZSTD(1)),
    data_type Enum('string', 'bool', 'int64', 'float64') CODEC(ZSTD(1)),
    string_value LowCardinality(String) CODEC(ZSTD(1))
) ENGINE {{.SIGNOZ_REPLICATED}}ReplacingMergeTree
ORDER BY (name, type, data_type, string_value)
TTL toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE
SETTINGS ttl_only_drop_parts = 1, allow_nullable_key = 1;

CREATE TABLE IF NOT EXISTS signoz_metrics.distributed_metric_labels ON CLUSTER {{.SIGNOZ_CLUSTER}} AS signoz_metrics.metric_labels ENGINE = Distributed("{{.SIGNOZ_CLUSTER}}", "signoz_metrics", metric_labels, rand());
