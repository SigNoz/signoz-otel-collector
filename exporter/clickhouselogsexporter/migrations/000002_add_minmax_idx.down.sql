alter table signoz_logs.logs ON CLUSTER signoz drop index id_minmax;

alter table signoz_logs.distributed_logs ON CLUSTER signoz drop index id_minmax;