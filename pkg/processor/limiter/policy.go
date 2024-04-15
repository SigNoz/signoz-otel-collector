package limiter

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/cache"
	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
)

const (
	PolicyPostgres = "postgres"
	PolicyRedis    = "redis"
)

func NewPostgresPolicy(storage *storage.Storage) entity.LimitMetricRepository {
	return storage.DAO.LimitMetrics()
}

func NewRedisPolicy(cache *cache.Cache) entity.LimitMetricRepository {
	return cache.CAO.LimitMetrics()
}
