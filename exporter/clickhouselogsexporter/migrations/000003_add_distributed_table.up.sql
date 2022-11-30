CREATE TABLE IF NOT EXISTS signoz_logs.distributed_logs  ON CLUSTER signoz AS signoz_logs.logs
ENGINE = Distributed("signoz", "signoz_logs", logs, cityHash64(id));

CREATE TABLE IF NOT EXISTS signoz_logs.distributed_logs_atrribute_keys  ON CLUSTER signoz AS signoz_logs.logs_atrribute_keys
ENGINE = Distributed("signoz", "signoz_logs", logs_atrribute_keys, cityHash64(datatype));

CREATE TABLE IF NOT EXISTS signoz_logs.distributed_logs_resource_keys  ON CLUSTER signoz AS signoz_logs.logs_resource_keys
ENGINE = Distributed("signoz", "signoz_logs", logs_resource_keys, cityHash64(datatype));

