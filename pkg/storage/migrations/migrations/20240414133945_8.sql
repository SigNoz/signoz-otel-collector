-- Modify "limit_metric" table
ALTER TABLE "limit_metric" DROP CONSTRAINT "limit_metric_pkey", DROP COLUMN "updated_at", ADD COLUMN "period_at" timestamptz NOT NULL, ADD PRIMARY KEY ("period", "signal", "period_at", "key_id");
