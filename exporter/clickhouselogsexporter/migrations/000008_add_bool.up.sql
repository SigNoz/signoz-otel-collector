ALTER TABLE signoz_logs.logs ON CLUSTER cluster add column IF NOT EXISTS attributes_bool_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER cluster add column IF NOT EXISTS attributes_bool_value Array(Bool) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER cluster add column IF NOT EXISTS attributes_bool_key Array(String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.distributed_logs ON CLUSTER cluster add column IF NOT EXISTS attributes_bool_value Array(Bool) CODEC(ZSTD(1));