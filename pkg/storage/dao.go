package storage

import (
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/off"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/postgres"
)

type DAO interface {
	Tenants() entity.TenantRepository
}

func NewDAO(strategy Strategy, opts options) DAO {
	switch strategy {
	case Off:
		return off.NewDAO()
	case Postgres:
		return postgres.NewDAO(opts.host, opts.port, opts.user, opts.password, opts.database)
	}

	panic(fmt.Sprintf("strategy %s has no dao implemented", strategy.String()))
}
