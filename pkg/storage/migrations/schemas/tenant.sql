CREATE TABLE "tenant" (
    "id" TEXT PRIMARY KEY,
    "name" TEXT UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL
);