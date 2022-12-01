CREATE TABLE IF NOT EXISTS signoz_traces.dependency_graph_minutes ON CLUSTER cluster (
    src LowCardinality(String) CODEC(ZSTD(1)),
    dest LowCardinality(String) CODEC(ZSTD(1)),
    duration_quantiles_state AggregateFunction(quantiles(0.5, 0.75, 0.9, 0.95, 0.99), Float64) CODEC(Default),
    error_count SimpleAggregateFunction(sum, UInt64) CODEC(T64, ZSTD(1)),
    total_count SimpleAggregateFunction(sum, UInt64) CODEC(T64, ZSTD(1)),
    timestamp DateTime CODEC(DoubleDelta, LZ4)
) ENGINE AggregatingMergeTree
PARTITION BY toDate(timestamp)
ORDER BY (timestamp, src, dest);



CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_service_calls_mv ON CLUSTER cluster
TO signoz_traces.dependency_graph_minutes AS
SELECT
    A.serviceName as src,
    B.serviceName as dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.durationNano)) as duration_quantiles_state,
    countIf(B.statusCode=2) as error_count,
    count(*) as total_count,
    toStartOfMinute(B.timestamp) as timestamp
FROM signoz_traces.signoz_index_v2 AS A, signoz_traces.signoz_index_v2 AS B
WHERE (A.serviceName != B.serviceName) AND (A.spanID = B.parentSpanID)
GROUP BY timestamp, src, dest;

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv ON CLUSTER cluster
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

CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv ON CLUSTER cluster
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
