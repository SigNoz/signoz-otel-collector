-- Please run the below commands if you are trying to fix schema migration issue https://signoz.io/docs/userguide/logs_troubleshooting/#schema-migrator-dirty-database-version
--ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'scope') CODEC(ZSTD(1))
--ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagType Enum8('tag', 'resource', 'scope') CODEC(ZSTD(1))

SELECT 1