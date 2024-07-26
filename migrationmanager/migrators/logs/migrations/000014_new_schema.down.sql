DROP TABLE IF EXISTS signoz_logs.logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};


ALTER TABLE signoz_logs.tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagDataType Enum('string', 'bool', 'int64', 'float64') CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_tag_attributes ON CLUSTER {{.SIGNOZ_CLUSTER}} modify column tagDataType Enum('string', 'bool', 'int64', 'float64') CODEC(ZSTD(1));