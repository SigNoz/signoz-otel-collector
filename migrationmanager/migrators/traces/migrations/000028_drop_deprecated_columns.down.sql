
DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} 
ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCMethod LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS httpCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS component LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} 
ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCMethod LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS httpCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS component LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} 
ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCMethod LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS httpCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS component LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}}
ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS gRPCMethod LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS httpCode LowCardinality(String) CODEC(ZSTD(1)),
ADD COLUMN IF NOT EXISTS component LowCardinality(String) CODEC(ZSTD(1));

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
    component,
    httpMethod,
    httpUrl,
    httpCode,
    httpRoute,
    httpHost,
    gRPCCode,
    gRPCMethod,
    hasError,
    tagMap,
    rpcSystem,
    rpcService,
    rpcMethod,
    responseStatusCode,
    stringTagMap,
    numberTagMap,
    boolTagMap,
    isRemote
FROM signoz_traces.signoz_index_v2
ORDER BY durationNano, timestamp;

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_traces.dependency_graph_minutes AS
SELECT
    serviceName as src,
    tagMap['db.system'] as dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
    countIf(statusCode=2) as error_count,
    count(*) as total_count,
    toStartOfMinute(timestamp) as timestamp
FROM signoz_traces.signoz_index_v2
WHERE dest != '' and kind != 2
GROUP BY timestamp, src, dest;

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_traces.dependency_graph_minutes AS
SELECT
    serviceName as src,
    tagMap['messaging.system'] as dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
    countIf(statusCode=2) as error_count,
    count(*) as total_count,
    toStartOfMinute(timestamp) as timestamp
FROM signoz_traces.signoz_index_v2
WHERE dest != '' and kind != 2
GROUP BY timestamp, src, dest;

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_traces.dependency_graph_minutes_v2 AS
SELECT
    serviceName as src,
    tagMap['db.system'] as dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
    countIf(statusCode=2) as error_count,
    count(*) as total_count,
    toStartOfMinute(timestamp) as timestamp,
    resourceTagsMap['deployment.environment'] as deployment_environment,
    resourceTagsMap['k8s.cluster.name'] as k8s_cluster_name,
    resourceTagsMap['k8s.namespace.name'] as k8s_namespace_name
FROM signoz_traces.signoz_index_v2
WHERE dest != '' and kind != 2
GROUP BY timestamp, src, dest, deployment_environment, k8s_cluster_name, k8s_namespace_name;

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_traces.dependency_graph_minutes_v2 AS
SELECT
    serviceName as src,
    tagMap['messaging.system'] as dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) as duration_quantiles_state,
    countIf(statusCode=2) as error_count,
    count(*) as total_count,
    toStartOfMinute(timestamp) as timestamp,
    resourceTagsMap['deployment.environment'] as deployment_environment,
    resourceTagsMap['k8s.cluster.name'] as k8s_cluster_name,
    resourceTagsMap['k8s.namespace.name'] as k8s_namespace_name
FROM signoz_traces.signoz_index_v2
WHERE dest != '' and kind != 2
GROUP BY timestamp, src, dest, deployment_environment, k8s_cluster_name, k8s_namespace_name;