ALTER TABLE signoz_logs.logs ON CLUSTER {{.SIGNOZ_CLUSTER}} DROP INDEX IF EXISTS id_minmax;
