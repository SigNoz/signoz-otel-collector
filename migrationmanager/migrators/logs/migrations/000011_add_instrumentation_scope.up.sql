ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_version String CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_version String CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_key Array(String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_value Array(String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS instrumentation_scope_attributes_string_value Array(String) CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS instrumentation_scope_idx (instrumentation_scope) TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4;