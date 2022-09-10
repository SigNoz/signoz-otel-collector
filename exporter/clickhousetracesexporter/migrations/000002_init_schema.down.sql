ALTER TABLE signoz_index ON CLUSTER signoz DROP COLUMN IF EXISTS events;
ALTER TABLE distributed_signoz_index DROP COLUMN IF EXISTS events;