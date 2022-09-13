ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER signoz
    DROP COLUMN IF EXISTS `testColumn`;

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER signoz
    DROP COLUMN IF EXISTS `testColumn`,