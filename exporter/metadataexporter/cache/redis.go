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

	debug bool
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

	Debug bool
}

var _ KeyCache = (*RedisKeyCache)(nil)

// 6-hour rolling window
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

		debug: opts.Debug,
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
		return sixHours
	}
}

func getCurrentEpochWindowMillis() int64 {
	return time.Now().UnixMilli() / sixHoursInMs * sixHoursInMs
}

func (c *RedisKeyCache) getAttrsKey(ds pipeline.Signal, resourceFpStr string) string {
	// e.g. "tenantID:metadata:traces:<6hWindow>:resource:<resourceFpStr>"
	return fmt.Sprintf("%s:metadata:%s:%d:resource:%s",
		c.tenantID, ds.String(), getCurrentEpochWindowMillis(), resourceFpStr)
}

// getResourcesHLLKey returns the HLL key for the set of resource fingerprints
// for the current 6h window for the given signal (for total unique resources)
func (c *RedisKeyCache) getResourcesHLLKey(ds pipeline.Signal) string {
	return fmt.Sprintf("%s:metadata:%s:%d:resources:hll",
		c.tenantID, ds.String(), getCurrentEpochWindowMillis())
}

// getAttrsHLLKey returns the HLL key for the set of attribute fingerprints
// for the current 6h window for the given signal (for total unique attributes)
func (c *RedisKeyCache) getAttrsHLLKey(ds pipeline.Signal) string {
	return fmt.Sprintf("%s:metadata:%s:%d:attrs:hll",
		c.tenantID, ds.String(), getCurrentEpochWindowMillis())
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

func (c *RedisKeyCache) getMaxTotalCardinality(ds pipeline.Signal) uint64 {
	switch ds {
	case pipeline.SignalTraces:
		return c.tracesMaxTotalCardinality
	case pipeline.SignalMetrics:
		return c.metricsMaxTotalCardinality
	case pipeline.SignalLogs:
		return c.logsMaxTotalCardinality
	}
	return 0
}

// AddAttrsToResource adds attrFps for the given resourceFp, respecting cardinalities.
func (c *RedisKeyCache) AddAttrsToResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) error {
	if len(attrFps) == 0 {
		return nil
	}

	resourceFpStr := strconv.FormatUint(resourceFp, 10)
	attrsKey := c.getAttrsKey(ds, resourceFpStr)
	ttl := c.getTTL(ds)

	pipe := c.redisClient.Pipeline()

	// check how many resources we *approx* have already via HLL:
	resourceHLLKey := c.getResourcesHLLKey(ds)
	currentCount, err := c.redisClient.PFCount(ctx, resourceHLLKey).Result()
	if err != nil {
		return err
	}
	if uint64(currentCount) >= c.getMaxResourceFp(ds) {
		return fmt.Errorf("too many resource fingerprints in %s cache (approx HLL limit)", ds.String())
	}
	// add resourceFp to the HLL
	pipe.PFAdd(ctx, resourceHLLKey, resourceFpStr)
	pipe.Expire(ctx, resourceHLLKey, ttl)

	// 2) Check how many attributes we have for this resource
	card, err := c.redisClient.SCard(ctx, attrsKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if card+int64(len(attrFps)) > int64(c.getMaxAttrs(ds)) {
		return fmt.Errorf("too many attribute fingerprints for resource %d in %s cache (card: %d, max: %d)",
			resourceFp, ds.String(), card+int64(len(attrFps)), c.getMaxAttrs(ds))
	}

	// Convert each attrFp to string
	members := make([]interface{}, len(attrFps))
	for i, a := range attrFps {
		members[i] = strconv.FormatUint(a, 10)
	}

	// 3) Add them to that resource’s set
	pipe.SAdd(ctx, attrsKey, members...)
	pipe.Expire(ctx, attrsKey, ttl)

	// 4) Also add them to the global attributes HLL
	attrsHLLKey := c.getAttrsHLLKey(ds)
	pipe.PFAdd(ctx, attrsHLLKey, members...)
	pipe.Expire(ctx, attrsHLLKey, ttl)

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

// ResourcesLimitExceeded uses PFCount on the resources HLL to do an approximate check.
func (c *RedisKeyCache) ResourcesLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	resourcesHLLKey := c.getResourcesHLLKey(ds)
	count, err := c.redisClient.PFCount(ctx, resourcesHLLKey).Result()
	if err != nil {
		c.logger.Error("failed to get resources HLL count", zap.Error(err), zap.String("datasource", ds.String()))
		return true
	}
	return uint64(count) >= c.getMaxResourceFp(ds)
}

// TotalCardinalityLimitExceeded uses PFCount on the attributes HLL to do an approximate check.
func (c *RedisKeyCache) TotalCardinalityLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	attrsHLLKey := c.getAttrsHLLKey(ds)
	count, err := c.redisClient.PFCount(ctx, attrsHLLKey).Result()
	if err != nil {
		c.logger.Error("failed to get attrs HLL count", zap.Error(err), zap.String("datasource", ds.String()))
		return true
	}
	return uint64(count) >= c.getMaxTotalCardinality(ds)
}

// CardinalityLimitExceeded is still an exact check for that single resource.
func (c *RedisKeyCache) CardinalityLimitExceeded(ctx context.Context, resourceFp uint64, ds pipeline.Signal) bool {
	resourceFpStr := strconv.FormatUint(resourceFp, 10)
	attrsKey := c.getAttrsKey(ds, resourceFpStr)
	card, err := c.redisClient.SCard(ctx, attrsKey).Result()
	if err != nil {
		return false
	}
	return uint64(card) >= c.getMaxAttrs(ds)
}

func (c *RedisKeyCache) CardinalityLimitExceededMulti(ctx context.Context, resourceFps []uint64, ds pipeline.Signal) ([]bool, error) {
	resourceFpStrs := make([]string, len(resourceFps))
	for i, resourceFp := range resourceFps {
		resourceFpStrs[i] = strconv.FormatUint(resourceFp, 10)
	}

	pipe := c.redisClient.Pipeline()
	for _, resourceFpStr := range resourceFpStrs {
		attrsKey := c.getAttrsKey(ds, resourceFpStr)
		pipe.SCard(ctx, attrsKey)
	}

	results, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]bool, len(resourceFps))
	for i, result := range results {
		cardCmd, ok := result.(*redis.IntCmd)
		if !ok {
			return nil, fmt.Errorf("expected *redis.IntCmd, got %T", result)
		}
		card, err := cardCmd.Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get cardinality for index %d: %w", i, err)
		}
		out[i] = uint64(card) >= c.getMaxAttrs(ds)
	}
	return out, nil
}

// Debug prints the keys that match the metadata pattern, plus each set’s cardinality.
// (Doesn’t print HLL counts, but you could add that easily with PFCount calls.)
func (c *RedisKeyCache) Debug(ctx context.Context) {
	if !c.debug {
		return
	}

	c.logger.Debug("DEBUGGING REDIS KEY CACHE")
	pattern := fmt.Sprintf("%s:metadata:*", c.tenantID)
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		c.logger.Error("failed to get keys", zap.Error(err))
		return
	}
	c.logger.Debug("keys", zap.Strings("keys", keys))

	for _, key := range keys {
		if err := c.debugKey(ctx, key); err != nil {
			c.logger.Error("failed to debug key", zap.String("key", key), zap.Error(err))
		}
	}
}

// debugKey is a helper function that tries SCARD if it’s a set, or PFCount if it’s a hll
func (c *RedisKeyCache) debugKey(ctx context.Context, key string) error {
	// If it ends with ":hll", assume it is a HyperLogLog key
	if len(key) >= 4 && key[len(key)-4:] == ":hll" {
		hllCount, err := c.redisClient.PFCount(ctx, key).Result()
		if err != nil {
			return err
		}
		c.logger.Debug("HLL cardinality", zap.String("key", key), zap.Int64("count", hllCount))
		return nil
	}

	// Otherwise try SCARD
	card, err := c.redisClient.SCard(ctx, key).Result()
	if err != nil {
		return err
	}
	c.logger.Debug("set cardinality", zap.String("key", key), zap.Int64("cardinality", card))
	return nil
}

func (c *RedisKeyCache) Close(ctx context.Context) error {
	if c.redisClient != nil {
		return c.redisClient.Close()
	}
	return nil
}
