-- Create "limit_metric" table
CREATE TABLE "limit_metric" (
 "period" text NOT NULL,
 "count" integer NOT NULL,
 "size" integer NOT NULL,
 "signal" text NOT NULL,
 "updated_at" timestamptz NOT NULL,
 "key_id" text NOT NULL,
 PRIMARY KEY ("period", "signal", "updated_at", "key_id")
);
-- Create "limit" table
CREATE TABLE "limit" (
 "id" text NOT NULL,
 "config" jsonb NOT NULL,
 "key_id" text NOT NULL,
 PRIMARY KEY ("id"),
 CONSTRAINT "key_id_fk" FOREIGN KEY ("key_id") REFERENCES "key" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
