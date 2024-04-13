-- Create "key" table
CREATE TABLE "key" (
 "id" text NOT NULL,
 "name" text NULL,
 "value" text NULL,
 "created_at" timestamptz NOT NULL,
 "expires_at" timestamptz NULL,
 "tenant_id" text NULL,
 PRIMARY KEY ("id"),
 CONSTRAINT "tenant_id_fk" FOREIGN KEY ("tenant_id") REFERENCES "tenant" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
