ALTER TABLE signoz_traces.signoz_index ON CLUSTER signoz ADD COLUMN IF NOT EXISTS hasError Int32;