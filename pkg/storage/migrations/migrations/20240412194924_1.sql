-- Create "tenant" table
CREATE TABLE "tenant" (
 "id" text NOT NULL,
 "name" text NULL,
 "created_at" timestamptz NOT NULL,
 PRIMARY KEY ("id")
);
-- Create index "tenant_name_key" to table: "tenant"
CREATE UNIQUE INDEX "tenant_name_key" ON "tenant" ("name");
