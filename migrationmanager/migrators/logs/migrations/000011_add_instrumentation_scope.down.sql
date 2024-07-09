ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource') CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource') CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP INDEX IF EXISTS scope_name_idx;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_name;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_name;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_version;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_version;


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_string_key;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_string_key;

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_string_value;
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP column IF EXISTS scope_string_value;