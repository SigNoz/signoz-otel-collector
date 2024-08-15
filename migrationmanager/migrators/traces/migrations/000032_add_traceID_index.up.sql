ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    ADD INDEX IF NOT EXISTS idx_traceID traceID TYPE tokenbf_v1(1024, 2, 0) GRANULARITY 1;