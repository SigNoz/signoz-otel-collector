DROP TABLE IF EXISTS signoz_logs.logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};