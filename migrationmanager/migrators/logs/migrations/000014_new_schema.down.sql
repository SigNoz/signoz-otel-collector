DROP TABLE IF EXISTS signoz_logs.resource_keys_string_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_float64_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_string_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.attribute_keys_bool_final_mv ON CLUSTER  {{.SIGNOZ_CLUSTER}};

CREATE MATERIALIZED VIEW IF NOT EXISTS  attribute_keys_string_final_mv ON CLUSTER {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(attributes_string_key) as name, 'String' datatype
FROM signoz_logs.logs
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  attribute_keys_int64_final_mv ON CLUSTER {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(attributes_int64_key) as name, 'Int64' datatype
FROM signoz_logs.logs
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  attribute_keys_float64_final_mv ON CLUSTER {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(attributes_float64_key) as name, 'Float64' datatype
FROM signoz_logs.logs
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  signoz_logs.attribute_keys_bool_final_mv ON CLUSTER {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_attribute_keys AS
SELECT
distinct arrayJoin(attributes_bool_key) as name, 'Bool' datatype
FROM signoz_logs.logs
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  resource_keys_string_final_mv  ON CLUSTER {{.SIGNOZ_CLUSTER}} TO signoz_logs.logs_resource_keys AS
SELECT
distinct arrayJoin(resources_string_key) as name, 'String' datatype
FROM signoz_logs.logs
ORDER BY  name;

DROP TABLE IF EXISTS signoz_logs.logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2_resource_bucket ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};
DROP TABLE IF EXISTS signoz_logs.distributed_logs_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}};
