DROP TABLE IF EXISTS signoz_traces.dependency_graph_minutes_v2 ON CLUSTER cluster;
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_service_calls_mv_v2 ON CLUSTER cluster;
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_db_calls_mv_v2 ON CLUSTER cluster;
DROP VIEW IF EXISTS signoz_traces.dependency_graph_minutes_messaging_calls_mv_v2 ON CLUSTER cluster;

DROP TABLE IF EXISTS signoz_traces.distributed_dependency_graph_minutes_v2 ON CLUSTER cluster;
