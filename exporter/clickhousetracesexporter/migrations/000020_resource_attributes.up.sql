ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS `resourceTagsMap` Map(LowCardinality(String), String) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS `resourceTagsMap` Map(LowCardinality(String), String) CODEC(ZSTD(1));