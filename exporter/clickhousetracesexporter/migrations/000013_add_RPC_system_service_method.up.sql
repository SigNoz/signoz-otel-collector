DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER signoz;

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.durationSort ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER signoz
    ADD INDEX IF NOT EXISTS idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;


ALTER TABLE signoz_traces.durationSort ON CLUSTER signoz
    ADD INDEX IF NOT EXISTS idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;


CREATE MATERIALIZED VIEW signoz_traces.durationSortMV ON CLUSTER signoz
TO signoz_traces.durationSort
AS SELECT
  timestamp,
  traceID,
  spanID,
  parentSpanID,
  serviceName,
  name,
  kind,
  durationNano,
  statusCode,
  component,
  httpMethod,
  httpUrl,
  httpCode,
  httpRoute,
  httpHost,
  gRPCMethod,
  gRPCCode,
  hasError,
  tagMap,
  rpcSystem,
  rpcService,
  rpcMethod,
  responseStatusCode
FROM signoz_traces.signoz_index_v2
ORDER BY durationNano, timestamp;
