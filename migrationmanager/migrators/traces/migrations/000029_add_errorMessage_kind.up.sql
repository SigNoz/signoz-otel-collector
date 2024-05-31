DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}};

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS errorMessage String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS errorMessage String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS errorMessage String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS errorMessage String CODEC(ZSTD(1));

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS spanKind String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS spanKind String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS spanKind String CODEC(ZSTD(1));
ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD COLUMN IF NOT EXISTS spanKind String CODEC(ZSTD(1));


ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS idx_spanKind spanKind TYPE set(5) GRANULARITY 4;


CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}}
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
    httpMethod,
    httpUrl,
    httpRoute,
    httpHost,
    hasError,
    rpcSystem,
    rpcService,
    rpcMethod,
    responseStatusCode,
    stringTagMap,
    numberTagMap,
    boolTagMap,
    isRemote,
    errorMessage,
    spanKind
FROM signoz_traces.signoz_index_v2
ORDER BY durationNano, timestamp;