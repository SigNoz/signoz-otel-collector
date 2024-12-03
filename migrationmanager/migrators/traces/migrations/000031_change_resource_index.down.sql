ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP INDEX IF EXISTS idx_resourceTagMapKeys,
    DROP INDEX IF EXISTS idx_resourceTagMapValues,

    ADD INDEX IF NOT EXISTS idx_resourceTagsMapKeys mapKeys(resourceTagsMap) TYPE bloom_filter(0.01) GRANULARITY 64,
    ADD INDEX IF NOT EXISTS idx_resourceTagsMapValues mapValues(resourceTagsMap) TYPE bloom_filter(0.01) GRANULARITY 64;


