
ALTER TABLE signoz_logs.logs ON CLUSTER cluster DROP column IF NOT EXISTS attributes_bool_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER cluster DROP column IF NOT EXISTS attributes_bool_value Array(Bool) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER cluster DROP column IF NOT EXISTS attributes_bool_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER cluster DROP column IF NOT EXISTS attributes_bool_value Array(Bool) CODEC(ZSTD(1));

DROP TABLE IF EXISTS signoz_logs.attribute_keys_bool_final_mv ON CLUSTER cluster;