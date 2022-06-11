CREATE TABLE IF NOT EXISTS signoz_traces.signoz_spans (
    timestamp DateTime64(9) CODEC(DoubleDelta, LZ4),
    traceID FixedString(32) CODEC(ZSTD(1)),
    model String CODEC(ZSTD(9))
) ENGINE MergeTree()
PARTITION BY toDate(timestamp)
ORDER BY traceID
SETTINGS index_granularity=1024