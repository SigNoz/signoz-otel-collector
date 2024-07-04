ALTER TABLE signoz_logs.logs DROP INDEX IF EXISTS instrumentation_scope_idx;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope to scope_name;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope to scope_name;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_version to scope_version;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_version to scope_version;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_attributes_string_key to scope_string_key;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_attributes_string_key to scope_string_key;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_attributes_string_value to scope_string_value;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} RENAME column IF EXISTS instrumentation_scope_attributes_string_value to scope_string_value;


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS scope_name_idx (scope_name) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4;