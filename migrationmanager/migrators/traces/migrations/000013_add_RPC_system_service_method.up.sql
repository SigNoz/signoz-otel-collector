DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}};

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));



ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD COLUMN IF NOT EXISTS `rpcSystem` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcService` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `rpcMethod` LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS `responseStatusCode` LowCardinality(String) CODEC(ZSTD(1));



ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD INDEX IF NOT EXISTS idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;


ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD INDEX IF NOT EXISTS idx_rpcMethod rpcMethod TYPE bloom_filter GRANULARITY 4,
    ADD INDEX IF NOT EXISTS idx_responseStatusCode responseStatusCode TYPE set(0) GRANULARITY 1;
