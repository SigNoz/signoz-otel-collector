package redis

import (
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	redisLib "github.com/redis/go-redis/v9"
)

type redis struct {
	cache *redisLib.Client
}

func NewCAO(host string, port int, user string, password string, database int) *redis {
	cache := redisLib.NewClient(&redisLib.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Username: user,
		Password: password,
		DB:       database,
	})
	return &redis{
		cache: cache,
	}
}

func (cao *redis) LimitMetrics() entity.LimitMetricRepository {
	return newLimitMetric(cao.cache)
}
