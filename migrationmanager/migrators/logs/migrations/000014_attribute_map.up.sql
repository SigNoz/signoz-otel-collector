ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_string Map(String, String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_string Map(String, String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_int64 Map(String, Int64) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_int64 Map(String, Int64) CODEC(ZSTD(1));


ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_float64 Map(String, Float64) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_float64 Map(String, Float64) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_bool Map(String, Bool) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS attributes_bool Map(String, Bool) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS resources_string Map(String, String) CODEC(ZSTD(1));
ALTER TABLE signoz_logs.distributed_logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD column IF NOT EXISTS resources_string Map(String, String) CODEC(ZSTD(1));

ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_string_idx_key mapKeys(attributes_string) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_string_idx_val mapValues(attributes_string) TYPE tokenbf_v1(5000, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_int64_idx_key mapKeys(attributes_int64) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_int64_idx_val mapValues(attributes_int64) TYPE minmax GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_float64_idx_key mapKeys(attributes_float64) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_float64_idx_val mapValues(attributes_float64) TYPE minmax GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS attributes_bool_idx_key mapKeys(attributes_bool) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS resources_string_idx_key mapKeys(resources_string) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS resources_string_idx_val mapValues(resources_string) TYPE tokenbf_v1(5000, 2, 0) GRANULARITY 1;
