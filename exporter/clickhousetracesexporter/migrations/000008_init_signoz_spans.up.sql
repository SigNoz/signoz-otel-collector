CREATE TABLE IF NOT EXISTS signoz_traces.signoz_spans ON CLUSTER signoz (
    timestamp DateTime64(9) CODEC(DoubleDelta, LZ4),
    traceID FixedString(32) CODEC(ZSTD(1)),
    model String CODEC(ZSTD(9))
) ENGINE ReplicatedMergeTree()
PARTITION BY toDate(timestamp)
ORDER BY traceID
SETTINGS index_granularity=1024;

CREATE TABLE distributed_signoz_spans ON CLUSTER signoz AS signoz_spans
ENGINE = Distributed("signoz", currentDatabase(), signoz_spans);