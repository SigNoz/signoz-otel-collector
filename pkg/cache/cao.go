package cache

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/cache/strategies/redis"
	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
)

type CAO interface {
	LimitMetrics() entity.LimitMetricRepository
}

func NewCAO(opts options) CAO {
	return redis.NewCAO(opts.host, opts.port, opts.user, opts.password, opts.database)
}
