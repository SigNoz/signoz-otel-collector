ALTER TABLE signoz_index DROP COLUMN IF EXISTS httpMethod, DROP COLUMN IF EXISTS httpUrl, DROP COLUMN IF EXISTS httpCode, DROP COLUMN IF EXISTS httpRoute, DROP COLUMN IF EXISTS httpHost, DROP COLUMN IF EXISTS msgSystem, DROP COLUMN IF EXISTS msgOperation;

ALTER TABLE distributed_signoz_index DROP COLUMN IF EXISTS httpMethod, DROP COLUMN IF EXISTS httpUrl, DROP COLUMN IF EXISTS httpCode, DROP COLUMN IF EXISTS httpRoute, DROP COLUMN IF EXISTS httpHost, DROP COLUMN IF EXISTS msgSystem, DROP COLUMN IF EXISTS msgOperation;