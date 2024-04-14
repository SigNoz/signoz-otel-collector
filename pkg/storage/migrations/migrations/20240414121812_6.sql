-- Modify "limit_metric" table
ALTER TABLE "limit_metric" DROP COLUMN "count", DROP COLUMN "size", ADD COLUMN "value" jsonb NOT NULL;
