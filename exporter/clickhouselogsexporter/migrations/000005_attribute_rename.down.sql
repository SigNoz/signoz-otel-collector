RENAME TABLE IF EXISTS signoz_logs.logs_attribute_keys TO signoz_logs.logs_atrribute_keys;


-- Materialized view
DROP VIEW IF EXISTS attribute_keys_string_final_mv;
DROP VIEW IF EXISTS attribute_keys_int64_final_mv;
DROP VIEW IF EXISTS attribute_keys_float64_final_mv;

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_string_final_mv ON CLUSTER cluster TO signoz_logs.logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_string_key) as name, 'String' datatype
FROM signoz_logs.logs
ORDER BY name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_int64_final_mv ON CLUSTER cluster TO signoz_logs.logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_int64_key) as name, 'Int64' datatype
FROM signoz_logs.logs
ORDER BY  name;

CREATE MATERIALIZED VIEW IF NOT EXISTS  atrribute_keys_float64_final_mv ON CLUSTER cluster TO signoz_logs.logs_atrribute_keys AS
SELECT
distinct arrayJoin(attributes_float64_key) as name, 'Float64' datatype
FROM signoz_logs.logs
ORDER BY  name;


-- Distributed table
DROP TABLE IF EXISTS signoz_logs.distributed_logs_attribute_keys ;

CREATE TABLE IF NOT EXISTS signoz_logs.distributed_logs_atrribute_keys  ON CLUSTER cluster AS signoz_logs.logs_atrribute_keys
ENGINE = Distributed("cluster", "signoz_logs", logs_atrribute_keys, cityHash64(datatype));