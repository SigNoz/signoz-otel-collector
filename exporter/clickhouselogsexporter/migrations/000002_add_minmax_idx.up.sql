alter table signoz_logs.logs ON CLUSTER signoz add index id_minmax id TYPE minmax GRANULARITY 1;