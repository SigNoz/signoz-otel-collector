package storage

import (
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies/off"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies/postgres"
)

type DAO interface {
	Tenants() entity.TenantRepository
	Keys() entity.KeyRepository
	Limits() entity.LimitRepository
	LimitMetrics() entity.LimitMetricRepository
}

func NewDAO(strategy strategies.Strategy, opts options) DAO {
	switch strategy {
	case strategies.Off:
		return off.NewDAO()
	case strategies.Postgres:
		return postgres.NewDAO(opts.host, opts.port, opts.user, opts.password, opts.database)
	default:
		panic(fmt.Sprintf("strategy %s has no dao implemented", strategy.String()))
	}

}
