ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP INDEX IF EXISTS instrumentation_scope_idx;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS instrumentation_scope;

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS instrumentation_scope;