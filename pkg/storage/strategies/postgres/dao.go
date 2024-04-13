package postgres

import (
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type postgres struct {
	db *sqlx.DB
}

func NewDAO(host string, port int, user string, password string, database string) *postgres {
	db := sqlx.MustConnect("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, database))
	return &postgres{
		db: db,
	}
}

func (dao *postgres) Tenants() entity.TenantRepository {
	return newTenant(dao.db)
}

func (dao *postgres) Keys() entity.KeyRepository {
	return newKey(dao.db)
}
