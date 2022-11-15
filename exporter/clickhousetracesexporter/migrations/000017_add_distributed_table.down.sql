DROP TABLE IF EXISTS signoz_traces.distributed_signoz_index ON CLUSTER signoz;
-- DROP TABLE IF EXISTS signoz_traces.distributed_schema_migrations ON CLUSTER signoz;

ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS events;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index ON CLUSTER signoz;

ALTER TABLE distributed_signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS httpMethod, DROP COLUMN IF EXISTS httpUrl, DROP COLUMN IF EXISTS httpCode, DROP COLUMN IF EXISTS httpRoute, DROP COLUMN IF EXISTS httpHost, DROP COLUMN IF EXISTS msgSystem, DROP COLUMN IF EXISTS msgOperation;

ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS hasError;
ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS tagMap;

DROP TABLE IF EXISTS signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_spans ON CLUSTER signoz;

DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index ON CLUSTER signoz;

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz DROP COLUMN IF EXISTS gRPCCode, DROP COLUMN IF EXISTS gRPCMethod;
DROP TABLE IF EXISTS signoz_traces.distributed_durationSort ON CLUSTER signoz;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_error_index_v2 ON CLUSTER signoz;

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz
    DROP COLUMN IF EXISTS `rpcSystem`,
    DROP COLUMN IF EXISTS `rpcService`,
    DROP COLUMN IF EXISTS `rpcMethod`,
    DROP COLUMN IF EXISTS `responseStatusCode`;


ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER signoz
    DROP COLUMN IF EXISTS `rpcSystem`,
    DROP COLUMN IF EXISTS `rpcService`,
    DROP COLUMN IF EXISTS `rpcMethod`,
    DROP COLUMN IF EXISTS `responseStatusCode`;

DROP TABLE IF EXISTS signoz_traces.distributed_usage_explorer ON CLUSTER signoz;

DROP TABLE IF EXISTS signoz_traces.distributed_top_level_operations ON CLUSTER signoz;

DROP TABLE IF EXISTS signoz_traces.distributed_dependency_graph_minutes ON CLUSTER signoz