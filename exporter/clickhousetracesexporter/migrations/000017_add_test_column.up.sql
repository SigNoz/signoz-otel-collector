ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `testColumn` LowCardinality(String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz
    ADD COLUMN IF NOT EXISTS `testColumn` LowCardinality(String) CODEC(ZSTD(1));