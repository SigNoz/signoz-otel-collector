DROP TABLE IF EXISTS signoz_traces.durationSortMV;

ALTER TABLE signoz_traces.signoz_index_v2
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.durationSort
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.signoz_index_v2
    ADD INDEX idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;

ALTER TABLE signoz_traces.durationSort
    ADD INDEX idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;

CREATE MATERIALIZED VIEW signoz_traces.durationSortMV
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
