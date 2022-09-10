alter table signoz_logs.logs ON CLUSTER signoz add index id_minmax id TYPE minmax GRANULARITY 1;

alter table signoz_logs.distributed_logs ON CLUSTER signoz add index id_minmax id TYPE minmax GRANULARITY 1;