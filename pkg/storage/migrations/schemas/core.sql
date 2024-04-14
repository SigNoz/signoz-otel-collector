-- TODO: Reset migrations to zero before merging to main
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

CREATE TABLE "limit" (
    "id" TEXT,
    "signal" TEXT NOT NULL,
    "config" JSONB NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL,
    "key_id" TEXT NOT NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "key_id_fk" FOREIGN KEY ("key_id") REFERENCES "key" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE TABLE "limit_metric" ( -- do not create foreign key on key_id
    "period" TEXT NOT NULL,
    "count" INTEGER NOT NULL,
    "size" INTEGER NOT NULL,
    "signal" TEXT NOT NULL,
    "period_at" TIMESTAMP WITH TIME ZONE NOT NULL,
    "key_id" TEXT NOT NULL,
    PRIMARY KEY ("period", "signal", "period_at", "key_id")
);
