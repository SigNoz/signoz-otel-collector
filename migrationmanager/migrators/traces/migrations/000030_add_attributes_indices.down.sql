ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP INDEX IF EXISTS idx_stringTagMapKeys,
    DROP INDEX IF EXISTS idx_stringTagMapValues,

    DROP INDEX IF EXISTS idx_numberTagMapKeys,
    DROP INDEX IF EXISTS idx_numberTagMapValues,

    DROP INDEX IF EXISTS idx_boolTagMapKeys;