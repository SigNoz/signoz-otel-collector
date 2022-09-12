CREATE TABLE IF NOT EXISTS signoz_traces.usage_explorer ON CLUSTER signoz (
  timestamp DateTime64(9) CODEC(DoubleDelta, LZ4),
  service_name LowCardinality(String) CODEC(ZSTD(1)),
  count UInt64 CODEC(T64, ZSTD(1))
) ENGINE ReplicatedSummingMergeTree('/clickhouse/tables/{cluster}/{shard}/signoz_traces/usage_explorer', '{replica}')
PARTITION BY toDate(timestamp)
ORDER BY (timestamp, service_name);

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_usage_explorer ON CLUSTER signoz AS signoz_traces.usage_explorer
ENGINE = Distributed("signoz", "signoz_traces", usage_explorer, cityHash64(service_name));

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.usage_explorer_mv ON CLUSTER signoz
TO signoz_traces.usage_explorer
AS SELECT
  toStartOfHour(timestamp) as timestamp,
  serviceName as service_name,
  count() as count
FROM signoz_traces.signoz_index_v2
GROUP BY timestamp, serviceName;