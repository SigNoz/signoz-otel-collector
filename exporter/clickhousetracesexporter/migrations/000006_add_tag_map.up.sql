ALTER TABLE signoz_traces.signoz_index ON CLUSTER signoz ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String);
ALTER TABLE signoz_traces.distributed_signoz_index ON CLUSTER signoz ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String);