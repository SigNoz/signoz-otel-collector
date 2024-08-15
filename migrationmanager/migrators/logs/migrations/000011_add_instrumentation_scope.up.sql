-- Please run the below commands if you are trying to fix schema migration issue https://signoz.io/docs/userguide/logs_troubleshooting/#schema-migrator-dirty-database-version
-- ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_version String CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_version String CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_key Array(String) CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_key Array(String) CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_value Array(String) CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_value Array(String) CODEC(ZSTD(1))
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS instrumentation_scope_idx (instrumentation_scope) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4


-- Don't run the commands below
ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'scope') CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'scope') CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_name String CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_name String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_version String CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_version String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_string_key Array(String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_string_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_string_value Array(String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS scope_string_value Array(String) CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS scope_name_idx (scope_name) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4;