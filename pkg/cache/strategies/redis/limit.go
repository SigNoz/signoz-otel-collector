package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	redisLib "github.com/redis/go-redis/v9"
)

var insert = redisLib.NewScript(`
local key = KEYS[1]
local count = tonumber(ARGV[1])
local size = tonumber(ARGV[2])
redis.call("hincrby", key, "count", count)
redis.call("hincrby", key, "size", size)
`)

type limitMetric struct {
	cache *redisLib.Client
}

func newLimitMetric(cache *redisLib.Client) entity.LimitMetricRepository {
	return &limitMetric{
		cache: cache,
	}
}

func (cao *limitMetric) Insert(ctx context.Context, limitMetrics []*entity.LimitMetric) error {
	cmds, err := cao.cache.Pipelined(ctx, func(pipe redisLib.Pipeliner) error {
		for _, limitMetric := range limitMetrics {
			insert.Eval(
				ctx,
				pipe,
				[]string{cao.key(ctx, limitMetric.Period, limitMetric.Signal, limitMetric.PeriodAt, limitMetric.KeyId)},
				limitMetric.Value.Count,
				limitMetric.Value.Size,
			)
		}
		return nil
	})
	if err != nil {
		// If the error is that the key does not exist ignore it,
		// because redis will create the key if it does not exist.
		if err == redisLib.Nil {
			return nil
		}
		return err
	}

	for _, cmd := range cmds {
		if err := cmd.(*redisLib.Cmd).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (cao *limitMetric) SelectValueByPeriodAndSignalAndPeriodAtAndKeyId(ctx context.Context, period entity.Period, signal entity.Signal, periodAt time.Time, keyId entity.Id) (entity.LimitValue, error) {
	limitValue := new(struct {
		Count int `redis:"count"`
		Size  int `redis:"size"`
	})
	err := cao.
		cache.
		HMGet(
			ctx,
			cao.key(ctx, period, signal, periodAt, keyId),
			"count",
			"size",
		).Scan(limitValue)
	if err != nil {
		if err == redisLib.Nil {
			return entity.LimitValue{}, errors.Newf(errors.TypeNotFound, "limit not found")
		}
		return entity.LimitValue{}, err
	}

	return entity.NewLimitValue(limitValue.Count, limitValue.Size), nil
}

func (cao *limitMetric) key(_ context.Context, period entity.Period, signal entity.Signal, periodAt time.Time, keyId entity.Id) string {
	return fmt.Sprintf(
		"signozlimit:%s:%s:%s:%s",
		signal.String(),
		keyId.String(),
		period.String(),
		periodAt.Format("2006-01-0215:04:05"),
	)
}
