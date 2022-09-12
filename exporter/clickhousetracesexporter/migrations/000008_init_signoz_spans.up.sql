CREATE TABLE IF NOT EXISTS signoz_traces.signoz_spans ON CLUSTER signoz (
    timestamp DateTime64(9) CODEC(DoubleDelta, LZ4),
    traceID FixedString(32) CODEC(ZSTD(1)),
    model String CODEC(ZSTD(9))
) ENGINE ReplicatedMergeTree('/clickhouse/tables/{cluster}/{shard}/signoz_traces/signoz_spans', '{replica}')
PARTITION BY toDate(timestamp)
ORDER BY traceID
SETTINGS index_granularity=1024;

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_signoz_spans ON CLUSTER signoz AS signoz_traces.signoz_spans
ENGINE = Distributed("signoz", "signoz_traces", signoz_spans, cityHash64(traceID));