CREATE TABLE IF NOT EXISTS signoz_traces.top_level_operations ON CLUSTER signoz (
    name LowCardinality(String) CODEC(ZSTD(1)),
    serviceName LowCardinality(String) CODEC(ZSTD(1))
) ENGINE ReplicatedReplacingMergeTree('/clickhouse/tables/{cluster}/{shard}/signoz_traces/top_level_operations', '{replica}')
ORDER BY (serviceName, name);

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_top_level_operations ON CLUSTER signoz AS signoz_traces.top_level_operations
ENGINE = Distributed("signoz", "signoz_traces", top_level_operations, cityHash64(serviceName));

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.sub_root_operations ON CLUSTER signoz 
TO signoz_traces.top_level_operations
AS SELECT DISTINCT
    name,
    serviceName
FROM signoz_traces.signoz_index_v2 AS A, signoz_traces.signoz_index_v2 AS B
WHERE (A.serviceName != B.serviceName) AND (A.parentSpanID = B.spanID);

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.root_operations ON CLUSTER signoz 
TO signoz_traces.top_level_operations
AS SELECT DISTINCT
    name,
    serviceName
FROM signoz_traces.signoz_index_v2
WHERE parentSpanID = '';
