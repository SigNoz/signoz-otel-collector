package postgres

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/jmoiron/sqlx"
)

type key struct {
	db *sqlx.DB
}

func newKey(db *sqlx.DB) entity.KeyRepository {
	return &key{
		db: db,
	}
}

func (dao *key) Insert(ctx context.Context, key *entity.Key) error {
	_, err := dao.
		db.
		ExecContext(
			ctx,
			"INSERT INTO key (id, name, value, created_at, expires_at, tenant_id) VALUES ($1, $2, $3, $4, $5, $6)",
			key.Id, key.Name, key.Value, key.CreatedAt, key.ExpiresAt, key.TenantId,
		)
	if err != nil {
		return err
	}

	return nil
}
