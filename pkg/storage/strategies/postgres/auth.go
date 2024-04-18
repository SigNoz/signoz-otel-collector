package postgres

import (
	"context"
	"database/sql"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/jmoiron/sqlx"
)

type auth struct {
	db *sqlx.DB
}

func newAuth(db *sqlx.DB) entity.AuthRepository {
	return &auth{
		db: db,
	}
}

func (dao *auth) SelectByKeyValue(ctx context.Context, keyValue string) (*entity.Auth, error) {
	key := new(entity.Key)
	tenant := new(entity.Tenant)
	err := dao.
		db.
		QueryRowContext(
			ctx,
			"SELECT key.*, tenant.* FROM key JOIN tenant ON key.tenant_id = tenant.id WHERE key.value = $1",
			keyValue,
		).Scan(&key.Id, &key.Name, &key.Value, &key.CreatedAt, &key.ExpiresAt, &key.TenantId, &tenant.Id, &tenant.Name, &tenant.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Newf(errors.TypeNotFound, "key not found")
		}
		return nil, err
	}

	return entity.NewAuth(key, tenant), nil
}
