ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS instrumentation_scope_idx (instrumentation_scope) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4;