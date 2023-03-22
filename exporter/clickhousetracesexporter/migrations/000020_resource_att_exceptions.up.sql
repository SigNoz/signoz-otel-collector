ALTER TABLE signoz_traces.signoz_error_index_v2 ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS resourceTagsMap Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ADD INDEX IF NOT EXISTS idx_resourceTagsMapKeys mapKeys(resourceTagsMap) TYPE bloom_filter(0.01) GRANULARITY 64,
    ADD INDEX IF NOT EXISTS idx_resourceTagsMapValues mapValues(resourceTagsMap) TYPE bloom_filter(0.01) GRANULARITY 64;

ALTER TABLE signoz_traces.distributed_signoz_error_index_v2 ON CLUSTER cluster
    ADD COLUMN IF NOT EXISTS resourceTagsMap Map(LowCardinality(String), String) CODEC(ZSTD(1));
