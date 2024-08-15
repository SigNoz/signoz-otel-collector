ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD INDEX IF NOT EXISTS idx_stringTagMapKeys mapKeys(stringTagMap) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1,
    ADD INDEX IF NOT EXISTS idx_stringTagMapValues mapValues(stringTagMap) TYPE ngrambf_v1(4, 5000, 2, 0) GRANULARITY 1,

    ADD INDEX IF NOT EXISTS idx_numberTagMapValues mapKeys(numberTagMap) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1,
    ADD INDEX IF NOT EXISTS idx_numberTagMapValues mapValues(numberTagMap) TYPE bloom_filter(0.01) GRANULARITY 1,

    ADD INDEX IF NOT EXISTS idx_boolTagMapKeys mapKeys(boolTagMap) TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;
