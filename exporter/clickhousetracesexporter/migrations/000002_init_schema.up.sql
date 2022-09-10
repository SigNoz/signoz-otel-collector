ALTER TABLE signoz_index ADD COLUMN IF NOT EXISTS events Array(String);
ALTER TABLE distributed_signoz_index ADD COLUMN IF EXISTS events Array(String);