CREATE TABLE IF NOT EXISTS signoz_traces.span_attributes ON CLUSTER cluster (
    timestamp DateTime CODEC(ZSTD(1)), 
    tagKey LowCardinality(String) CODEC(ZSTD(1)),
    tagType LowCardinality(String) CODEC(ZSTD(1)),
    stringTagValue String CODEC(ZSTD(1)),
    numberTagValue Nullable(Float64) CODEC(ZSTD(1)),
    boolTagValue Nullable(bool) CODEC(ZSTD(1)),
) ENGINE ReplacingMergeTree
ORDER BY (tagKey, tagType, stringTagValue, numberTagValue, boolTagValue)
TTL toDateTime(timestamp) + INTERVAL 86400 SECOND DELETE
SETTINGS ttl_only_drop_parts = 1, allow_nullable_key = 1;

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_span_attributes ON CLUSTER cluster AS signoz_traces.span_attributes
ENGINE = Distributed("cluster", "signoz_traces", span_attributes, cityHash64(tagKey));
