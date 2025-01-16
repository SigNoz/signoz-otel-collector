package metadataexporter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type RedisKeyCache struct {
	redisClient *redis.Client
	tenantID    string
	logger      *zap.Logger

	tracesTTL  time.Duration
	metricsTTL time.Duration
	logsTTL    time.Duration

	// Max # of resource fingerprints in each pipeline
	maxTracesResourceFp  uint64
	maxMetricsResourceFp uint64
	maxLogsResourceFp    uint64

	// Max # of attribute fingerprints per resource
	maxTracesCardinalityPerResource  uint64
	maxMetricsCardinalityPerResource uint64
	maxLogsCardinalityPerResource    uint64

	// Max # of attribute fingerprints in total
	tracesMaxTotalCardinality  uint64
	metricsMaxTotalCardinality uint64
	logsMaxTotalCardinality    uint64
}

type RedisKeyCacheOptions struct {
	Addr     string
	Username string
	Password string
	DB       int

	TenantID string
	Logger   *zap.Logger

	TracesTTL  time.Duration
	MetricsTTL time.Duration
	LogsTTL    time.Duration

	// Limits
	MaxTracesResourceFp  uint64
	MaxMetricsResourceFp uint64
	MaxLogsResourceFp    uint64

	MaxTracesCardinalityPerResource  uint64
	MaxMetricsCardinalityPerResource uint64
	MaxLogsCardinalityPerResource    uint64

	TracesMaxTotalCardinality  uint64
	MetricsMaxTotalCardinality uint64
	LogsMaxTotalCardinality    uint64
}

var _ KeyCache = (*RedisKeyCache)(nil)

const (
	sixHours     = 6 * time.Hour
	sixHoursInMs = int64(sixHours / time.Millisecond)
)

func NewRedisKeyCache(opts RedisKeyCacheOptions) (*RedisKeyCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Username: opts.Username,
		Password: opts.Password,
		DB:       opts.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &RedisKeyCache{
		redisClient: client,
		tenantID:    opts.TenantID,
		logger:      opts.Logger,

		tracesTTL:  opts.TracesTTL,
		metricsTTL: opts.MetricsTTL,
		logsTTL:    opts.LogsTTL,

		maxTracesResourceFp:  opts.MaxTracesResourceFp,
		maxMetricsResourceFp: opts.MaxMetricsResourceFp,
		maxLogsResourceFp:    opts.MaxLogsResourceFp,

		maxTracesCardinalityPerResource:  opts.MaxTracesCardinalityPerResource,
		maxMetricsCardinalityPerResource: opts.MaxMetricsCardinalityPerResource,
		maxLogsCardinalityPerResource:    opts.MaxLogsCardinalityPerResource,

		tracesMaxTotalCardinality:  opts.TracesMaxTotalCardinality,
		metricsMaxTotalCardinality: opts.MetricsMaxTotalCardinality,
		logsMaxTotalCardinality:    opts.LogsMaxTotalCardinality,
	}, nil
}

func (c *RedisKeyCache) getTTL(ds pipeline.Signal) time.Duration {
	switch ds {
	case pipeline.SignalTraces:
		return c.tracesTTL
	case pipeline.SignalMetrics:
		return c.metricsTTL
	case pipeline.SignalLogs:
		return c.logsTTL
	default:
		return 6 * time.Hour
	}
}

func getCurrentEpochWindowMillis() int64 {
	return time.Now().UnixMilli() / sixHoursInMs * sixHoursInMs
}

// getResourceSetKey returns the key for the set of resource fingerprints
// for the current 6h window for the given signal
func (c *RedisKeyCache) getResourceSetKey(ds pipeline.Signal) string {
	// e.g. "tenantID:metadata:traces:<6hWindow>:resources"
	return fmt.Sprintf("%s:metadata:%s:%d:resources",
		c.tenantID, ds.String(), getCurrentEpochWindowMillis())
}

func (c *RedisKeyCache) getAttrsKey(ds pipeline.Signal, resourceFpStr string) string {
	// e.g. "tenantID:metadata:traces:<6hWindow>:resource:<resourceFpStr>"
	return fmt.Sprintf("%s:metadata:%s:%d:resource:%s",
		c.tenantID, ds.String(), getCurrentEpochWindowMillis(), resourceFpStr)
}

func (c *RedisKeyCache) getMaxResourceFp(ds pipeline.Signal) uint64 {
	switch ds {
	case pipeline.SignalTraces:
		return c.maxTracesResourceFp
	case pipeline.SignalMetrics:
		return c.maxMetricsResourceFp
	case pipeline.SignalLogs:
		return c.maxLogsResourceFp
	}
	return 0
}

func (c *RedisKeyCache) getMaxAttrs(ds pipeline.Signal) uint64 {
	switch ds {
	case pipeline.SignalTraces:
		return c.maxTracesCardinalityPerResource
	case pipeline.SignalMetrics:
		return c.maxMetricsCardinalityPerResource
	case pipeline.SignalLogs:
		return c.maxLogsCardinalityPerResource
	}
	return 0
}

// AddAttrsToResource adds attrFps for the given resourceFp, respecting cardinalities.
func (c *RedisKeyCache) AddAttrsToResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) error {
	if len(attrFps) == 0 {
		return nil
	}

	resourceFpStr := strconv.FormatUint(resourceFp, 10)
	resourceSetKey := c.getResourceSetKey(ds)
	attrsKey := c.getAttrsKey(ds, resourceFpStr)
	ttl := c.getTTL(ds)

	pipe := c.redisClient.Pipeline()

	// 1) If resourceFp is not in resourceSetKey, check cardinality
	isMember, err := c.redisClient.SIsMember(ctx, resourceSetKey, resourceFpStr).Result()
	if err != nil {
		return err
	}
	if !isMember {
		// check how many resources
		card, err := c.redisClient.SCard(ctx, resourceSetKey).Result()
		if err != nil {
			return err
		}
		if card >= int64(c.getMaxResourceFp(ds)) {
			return fmt.Errorf("too many resource fingerprints in %s cache", ds.String())
		}
		pipe.SAdd(ctx, resourceSetKey, resourceFpStr)
		pipe.Expire(ctx, resourceSetKey, ttl)
	}

	// 2) Check how many attributes we already have for this resource
	card, err := c.redisClient.SCard(ctx, attrsKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if card+int64(len(attrFps)) > int64(c.getMaxAttrs(ds)) {
		return fmt.Errorf("too many attribute fingerprints for resource %d in %s cache",
			resourceFp, ds.String())
	}

	// Convert each attrFp to string
	members := make([]interface{}, len(attrFps))
	for i, a := range attrFps {
		members[i] = strconv.FormatUint(a, 10)
	}

	// 3) Add them all
	pipe.SAdd(ctx, attrsKey, members...)
	pipe.Expire(ctx, attrsKey, ttl)

	_, err = pipe.Exec(ctx)
	return err
}

// AttrsExistForResource checks if each attrFp is in that resource’s set.
func (c *RedisKeyCache) AttrsExistForResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) ([]bool, error) {
	if len(attrFps) == 0 {
		return nil, nil
	}
	resourceFpStr := strconv.FormatUint(resourceFp, 10)
	attrsKey := c.getAttrsKey(ds, resourceFpStr)

	members := make([]interface{}, len(attrFps))
	for i, a := range attrFps {
		members[i] = strconv.FormatUint(a, 10)
	}

	results, err := c.redisClient.SMIsMember(ctx, attrsKey, members...).Result()
	if err != nil {
		return nil, err
	}
	// results is a slice of bool, same length as members
	return results, nil
}

func (c *RedisKeyCache) ResourcesLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	resourceSetKey := c.getResourceSetKey(ds)
	card, err := c.redisClient.SCard(ctx, resourceSetKey).Result()
	if err != nil {
		return false
	}
	return uint64(card) >= c.getMaxResourceFp(ds)
}

func (c *RedisKeyCache) TotalCardinalityLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	// Get the base key for resource set
	resourceSetKey := c.getResourceSetKey(ds)

	var cursor uint64
	var totalCardinality uint64

	pipe := c.redisClient.Pipeline()
	for {
		keys, nextCursor, err := c.redisClient.Scan(ctx, cursor, resourceSetKey, 512).Result()
		if err != nil {
			return false
		}

		cmds := make([]*redis.IntCmd, len(keys))
		for i, resourceFp := range keys {
			attrsKey := c.getAttrsKey(ds, resourceFp)
			cmds[i] = pipe.SCard(ctx, attrsKey)
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return false
		}

		// Sum up the cardinalities from this batch
		for _, cmd := range cmds {
			card, err := cmd.Result()
			if err != nil {
				return false
			}
			totalCardinality += uint64(card)

			// Early return if limit is already exceeded
			if totalCardinality >= c.maxTracesCardinalityPerResource {
				return true
			}
		}

		// Update cursor for next iteration
		cursor = nextCursor

		// Break if we've scanned all keys
		if cursor == 0 {
			break
		}
	}

	return totalCardinality >= c.maxTracesCardinalityPerResource
}

func (c *RedisKeyCache) CardinalityLimitExceeded(ctx context.Context, resourceFp uint64, ds pipeline.Signal) bool {
	resourceFpStr := strconv.FormatUint(resourceFp, 10)
	attrsKey := c.getAttrsKey(ds, resourceFpStr)
	card, err := c.redisClient.SCard(ctx, attrsKey).Result()
	if err != nil {
		return false
	}
	return uint64(card) >= c.getMaxAttrs(ds)
}

func (c *RedisKeyCache) Debug(ctx context.Context) {
	c.logger.Debug("DEBUGGING REDIS KEY CACHE")
	keys, err := c.redisClient.Keys(ctx, fmt.Sprintf("%s:metadata:*", c.tenantID)).Result()
	if err != nil {
		c.logger.Error("failed to get keys", zap.Error(err))
		return
	}
	c.logger.Debug("keys", zap.Strings("keys", keys))

	for _, key := range keys {
		card, err := c.redisClient.SCard(ctx, key).Result()
		if err != nil {
			c.logger.Error("failed to get cardinality", zap.String("key", key), zap.Error(err))
			continue
		}
		c.logger.Debug("cardinality", zap.String("key", key), zap.Int64("cardinality", card))
	}
}
