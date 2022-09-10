DROP TABLE IF EXISTS resource_keys_string_final_mv ON CLUSTER signoz;
DROP TABLE IF EXISTS atrribute_keys_float64_final_mv ON CLUSTER signoz;
DROP TABLE IF EXISTS atrribute_keys_int64_final_mv ON CLUSTER signoz;
DROP TABLE IF EXISTS atrribute_keys_string_final_mv ON CLUSTER signoz;
DROP TABLE IF EXISTS logs_resource_keys ON CLUSTER signoz;
DROP TABLE IF EXISTS logs_atrribute_keys ON CLUSTER signoz;
DROP TABLE IF EXISTS logs ON CLUSTER signoz;


DROP TABLE IF EXISTS distributed_logs ON CLUSTER signoz;
DROP TABLE IF EXISTS distributed_logs_atrribute_keys ON CLUSTER signoz;
DROP TABLE IF EXISTS distributed_logs_resource_keys ON CLUSTER signoz;