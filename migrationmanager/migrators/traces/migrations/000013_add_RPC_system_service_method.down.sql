DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}};

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP INDEX idx_rpcMethod,
    DROP INDEX idx_responseStatusCode;

ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP INDEX idx_rpcMethod,
    DROP INDEX idx_responseStatusCode;
