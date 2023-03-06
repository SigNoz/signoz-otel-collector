DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER cluster;

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER cluster 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.durationSort ON CLUSTER cluster 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER cluster
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.durationSortMV ON CLUSTER cluster
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