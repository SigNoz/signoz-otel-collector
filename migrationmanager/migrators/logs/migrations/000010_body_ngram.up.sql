-- Please run the below commands if you are trying to fix schema migration issue https://signoz.io/docs/userguide/logs_troubleshooting/#schema-migrator-dirty-database-version
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP INDEX IF EXISTS body_idx
-- ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} ADD INDEX IF NOT EXISTS body_idx lower(body) TYPE ngrambf_v1(4, 60000, 5, 0) GRANULARITY 1SELECT 1

SELECT 1