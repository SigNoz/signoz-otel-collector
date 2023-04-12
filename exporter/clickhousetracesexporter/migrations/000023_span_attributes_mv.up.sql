CREATE TABLE IF NOT EXISTS signoz_traces.span_attributes ON CLUSTER cluster (
    timestamp DateTime CODEC(DoubleDelta, ZSTD(1)), 
    tagKey LowCardinality(String) CODEC(ZSTD(1)),
    tagType Enum('tag', 'resource') CODEC(ZSTD(1)),
    dataType Enum('string', 'bool', 'number') CODEC(ZSTD(1)),
    stringTagValue String CODEC(ZSTD(1)),
    numberTagValue Nullable(Float64) CODEC(ZSTD(1)),
    isColumn bool CODEC(ZSTD(1)),
) ENGINE ReplacingMergeTree
ORDER BY (tagKey, tagType, dataType, stringTagValue, numberTagValue)
TTL toDateTime(timestamp) + INTERVAL 172800 SECOND DELETE
SETTINGS ttl_only_drop_parts = 1, allow_nullable_key = 1;

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_span_attributes ON CLUSTER cluster AS signoz_traces.span_attributes
ENGINE = Distributed("cluster", "signoz_traces", span_attributes, cityHash64(rand()));
