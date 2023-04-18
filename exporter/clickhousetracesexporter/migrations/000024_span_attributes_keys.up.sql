CREATE TABLE IF NOT EXISTS signoz_traces.span_attributes_keys ON CLUSTER cluster (
    tagKey LowCardinality(String) CODEC(ZSTD(1)),
    tagType Enum('tag', 'resource') CODEC(ZSTD(1)),
    dataType Enum('string', 'bool', 'float64') CODEC(ZSTD(1)),
    isColumn bool CODEC(ZSTD(1)),
) ENGINE ReplacingMergeTree
ORDER BY (tagKey, tagType, dataType, isColumn);

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_span_attributes_keys ON CLUSTER cluster AS signoz_traces.span_attributes_keys
ENGINE = Distributed("cluster", "signoz_traces", span_attributes_keys, cityHash64(rand()));
