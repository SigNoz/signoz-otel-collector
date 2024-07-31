--ALTER TABLE signoz_logs.logs DROP INDEX IF EXISTS scope_name_idx
--ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_name to instrumentation_scope
--ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_name to instrumentation_scope
--ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_version to instrumentation_scope_version
--ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_version to instrumentation_scope_version
--ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_string_key to instrumentation_scope_attributes_string_key
--ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_string_key to instrumentation_scope_attributes_string_key
--ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_string_value to instrumentation_scope_attributes_string_value
--ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS scope_string_value to instrumentation_scope_attributes_string_value
--ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS instrumentation_scope_idx (instrumentation_scope) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4

SELECT 1