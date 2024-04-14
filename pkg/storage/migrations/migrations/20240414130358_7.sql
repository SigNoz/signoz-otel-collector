-- Modify "limit_metric" table
ALTER TABLE "limit_metric" DROP COLUMN "value", ADD COLUMN "count" integer NOT NULL, ADD COLUMN "size" integer NOT NULL;
