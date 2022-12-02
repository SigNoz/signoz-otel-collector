ALTER TABLE signoz_logs.logs ON CLUSTER cluster ADD INDEX IF NOT EXISTS id_minmax id TYPE minmax GRANULARITY 1;
