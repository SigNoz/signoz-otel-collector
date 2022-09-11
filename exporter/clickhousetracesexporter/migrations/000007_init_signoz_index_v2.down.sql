DROP TABLE IF EXISTS signoz_traces.signoz_index_v2 ON CLUSTER signoz;
DROP TABLE IF EXISTS signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz;

SET allow_experimental_projection_optimization = 0;