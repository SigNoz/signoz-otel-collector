CREATE TABLE "tenant" (
    "id" TEXT PRIMARY KEY,
    "name" TEXT UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL
);

CREATE TABLE "key" (
    "id" TEXT,
    "name" TEXT,
    "value" TEXT,
    "created_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NULL,
    "tenant_id" TEXT,
    PRIMARY KEY ("id"),
    CONSTRAINT "tenant_id_fk" FOREIGN KEY ("tenant_id") REFERENCES "tenant" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);