package postgres

import (
	"context"
	"database/sql"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/jmoiron/sqlx"
)

type tenant struct {
	db *sqlx.DB
}

func newTenant(db *sqlx.DB) entity.TenantRepository {
	return &tenant{
		db: db,
	}
}

func (dao *tenant) Insert(ctx context.Context, tenant *entity.Tenant) error {
	_, err := dao.
		db.
		ExecContext(
			ctx,
			"INSERT INTO tenant (id, name, created_at) VALUES ($1, $2, $3)",
			tenant.Id, tenant.Name, tenant.CreatedAt,
		)
	if err != nil {
		return err
	}

	return nil
}

func (dao *tenant) SelectByName(ctx context.Context, name string) (*entity.Tenant, error) {
	tenant := new(entity.Tenant)
	err := dao.
		db.
		QueryRowContext(
			ctx,
			"SELECT * FROM tenant WHERE name = ?",
			name,
		).Scan(tenant.Id, tenant.Name, tenant.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Newf(errors.TypeInternal, "tenant: %s not found", name)
		}
		return nil, err
	}

	return tenant, nil
}
