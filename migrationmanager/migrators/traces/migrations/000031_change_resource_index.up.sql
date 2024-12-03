ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP INDEX IF EXISTS idx_resourceTagsMapKeys,
    DROP INDEX IF EXISTS idx_resourceTagsMapValues,

    ADD INDEX IF NOT EXISTS idx_resourceTagMapKeys mapKeys(resourceTagsMap) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1,
    ADD INDEX IF NOT EXISTS idx_resourceTagMapValues mapValues(resourceTagsMap) TYPE ngrambf_v1(4, 5000, 2, 0) GRANULARITY 1;
