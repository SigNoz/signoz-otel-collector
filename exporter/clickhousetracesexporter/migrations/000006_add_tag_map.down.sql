ALTER TABLE signoz_traces.signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS tagMap;
ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS tagMap;