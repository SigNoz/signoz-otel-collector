CREATE TABLE IF NOT EXISTS signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster AS signoz_traces.signoz_index_v2
ENGINE = Distributed("cluster", "signoz_traces", signoz_index_v2, cityHash64(traceID));

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_signoz_spans ON CLUSTER cluster AS signoz_traces.signoz_spans
ENGINE = Distributed("cluster", "signoz_traces", signoz_spans, cityHash64(traceID));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster ADD COLUMN IF NOT EXISTS gRPCMethod LowCardinality(String) CODEC(ZSTD(1)), ADD COLUMN IF NOT EXISTS gRPCCode LowCardinality(String) CODEC(ZSTD(1));

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_durationSort ON CLUSTER cluster AS signoz_traces.durationSort
ENGINE = Distributed("cluster", "signoz_traces", durationSort, cityHash64(serviceName));
CREATE TABLE IF NOT EXISTS signoz_traces.distributed_signoz_error_index_v2 ON CLUSTER cluster AS signoz_traces.signoz_error_index_v2
ENGINE = Distributed("cluster", "signoz_traces", signoz_error_index_v2, cityHash64(groupID));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));


CREATE TABLE IF NOT EXISTS signoz_traces.distributed_usage_explorer ON CLUSTER cluster AS signoz_traces.usage_explorer
ENGINE = Distributed("cluster", "signoz_traces", usage_explorer, cityHash64(service_name));

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_top_level_operations ON CLUSTER cluster AS signoz_traces.top_level_operations
ENGINE = Distributed("cluster", "signoz_traces", top_level_operations, cityHash64(serviceName));

CREATE TABLE IF NOT EXISTS signoz_traces.distributed_dependency_graph_minutes ON CLUSTER cluster AS signoz_traces.dependency_graph_minutes
ENGINE = Distributed("cluster", "signoz_traces", dependency_graph_minutes, cityHash64(src, dest));

