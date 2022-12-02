DROP TABLE IF EXISTS signoz_traces.schema_migrations ON CLUSTER cluster;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_index ON CLUSTER cluster;
-- DROP TABLE IF EXISTS signoz_traces.distributed_schema_migrations ON CLUSTER cluster;

ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER cluster DROP COLUMN IF EXISTS events;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index ON CLUSTER cluster;

ALTER TABLE distributed_signoz_index ON CLUSTER cluster DROP COLUMN IF EXISTS httpMethod, DROP COLUMN IF EXISTS httpUrl, DROP COLUMN IF EXISTS httpCode, DROP COLUMN IF EXISTS httpRoute, DROP COLUMN IF EXISTS httpHost, DROP COLUMN IF EXISTS msgSystem, DROP COLUMN IF EXISTS msgOperation;

ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER cluster DROP COLUMN IF EXISTS hasError;
ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER cluster DROP COLUMN IF EXISTS tagMap;

DROP TABLE IF EXISTS signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_spans ON CLUSTER cluster;

DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index ON CLUSTER cluster;

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster DROP COLUMN IF EXISTS gRPCCode, DROP COLUMN IF EXISTS gRPCMethod;
DROP TABLE IF EXISTS signoz_traces.distributed_durationSort ON CLUSTER cluster;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index_v2 ON CLUSTER cluster;

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster
    DROP COLUMN IF EXISTS `rpcSystem`,
    DROP COLUMN IF EXISTS `rpcService`,
    DROP COLUMN IF EXISTS `rpcMethod`,
    DROP COLUMN IF EXISTS `responseStatusCode`;


ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER cluster
    DROP COLUMN IF EXISTS `rpcSystem`,
    DROP COLUMN IF EXISTS `rpcService`,
    DROP COLUMN IF EXISTS `rpcMethod`,
    DROP COLUMN IF EXISTS `responseStatusCode`;

DROP TABLE IF EXISTS signoz_traces.distributed_usage_explorer ON CLUSTER cluster;

DROP TABLE IF EXISTS signoz_traces.distributed_top_level_operations ON CLUSTER cluster;

DROP TABLE IF EXISTS signoz_traces.distributed_dependency_graph_minutes ON CLUSTER cluster