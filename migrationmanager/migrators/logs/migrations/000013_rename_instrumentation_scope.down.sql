--ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1))
--ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'instrumentation_scope') CODEC(ZSTD(1))

SELECT 1