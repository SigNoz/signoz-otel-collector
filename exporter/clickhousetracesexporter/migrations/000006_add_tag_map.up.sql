ALTER TABLE signoz_traces.signoz_index ON CLUSTER cluster ADD COLUMN IF NOT EXISTS tagMap Map(LowCardinality(String), String);