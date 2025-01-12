package metadataexporter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func getKey(tenantID string, ds pipeline.Signal) string {
	currentEpochWindowMillis := (time.Now().UnixMilli() / sixHoursInMs) * sixHoursInMs
	var key strings.Builder
	key.WriteString(tenantID)
	key.WriteString(":metadata:")
	key.WriteString(ds.String())
	key.WriteString(":")
	key.WriteString(strconv.FormatInt(currentEpochWindowMillis, 10))
	return key.String()
}

type KeyCache interface {
	ExistsMulti(ctx context.Context, keys []string, ds pipeline.Signal) ([]bool, error)
	AddMulti(ctx context.Context, keys []string, ds pipeline.Signal) error
	Add(ctx context.Context, key string, ds pipeline.Signal) error
	Exists(ctx context.Context, key string, ds pipeline.Signal) (bool, error)
	Debug(ctx context.Context)
}

type InMemoryKeyCacheOptions struct {
	TracesFingerprintCacheSize  int
	MetricsFingerprintCacheSize int
	LogsFingerprintCacheSize    int
	TracesFingerprintCacheTTL   time.Duration
	MetricsFingerprintCacheTTL  time.Duration
	LogsFingerprintCacheTTL     time.Duration
	TenantID                    string
	Logger                      *zap.Logger
}

type InMemoryKeyCache struct {
	tracesFingerprintCache  *ttlcache.Cache[string, struct{}]
	metricsFingerprintCache *ttlcache.Cache[string, struct{}]
	logsFingerprintCache    *ttlcache.Cache[string, struct{}]

	tenantID string

	logger *zap.Logger
}

func NewInMemoryKeyCache(opts InMemoryKeyCacheOptions) InMemoryKeyCache {
	tracesFingerprintCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](opts.TracesFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](uint64(opts.TracesFingerprintCacheSize)),
	)
	go tracesFingerprintCache.Start()

	metricsFingerprintCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](opts.MetricsFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](uint64(opts.MetricsFingerprintCacheSize)),
	)
	go metricsFingerprintCache.Start()

	logsFingerprintCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](opts.LogsFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](uint64(opts.LogsFingerprintCacheSize)),
	)
	go logsFingerprintCache.Start()

	return InMemoryKeyCache{
		tracesFingerprintCache:  tracesFingerprintCache,
		metricsFingerprintCache: metricsFingerprintCache,
		logsFingerprintCache:    logsFingerprintCache,
		tenantID:                opts.TenantID,
		logger:                  opts.Logger,
	}
}

func getKeyForInMemory(tenantID string, ds pipeline.Signal, key string) string {
	var newKey strings.Builder
	newKey.WriteString(getKey(tenantID, ds))
	newKey.WriteString(":")
	newKey.WriteString(key)
	return newKey.String()
}

func (c InMemoryKeyCache) Add(ctx context.Context, key string, ds pipeline.Signal) error {
	key = getKeyForInMemory(c.tenantID, ds, key)
	switch ds {
	case pipeline.SignalTraces:
		c.tracesFingerprintCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
	case pipeline.SignalMetrics:
		c.metricsFingerprintCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
	case pipeline.SignalLogs:
		c.logsFingerprintCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
	}
	return nil
}

func (c InMemoryKeyCache) Exists(ctx context.Context, key string, ds pipeline.Signal) (bool, error) {
	key = getKeyForInMemory(c.tenantID, ds, key)
	switch ds {
	case pipeline.SignalTraces:
		return c.tracesFingerprintCache.Get(key) != nil, nil
	case pipeline.SignalMetrics:
		return c.metricsFingerprintCache.Get(key) != nil, nil
	case pipeline.SignalLogs:
		return c.logsFingerprintCache.Get(key) != nil, nil
	}
	return false, nil
}

func (c InMemoryKeyCache) ExistsMulti(ctx context.Context, keys []string, ds pipeline.Signal) ([]bool, error) {
	exists := make([]bool, len(keys))
	for i, key := range keys {
		exists[i], _ = c.Exists(ctx, key, ds)
	}
	return exists, nil
}

func (c InMemoryKeyCache) AddMulti(ctx context.Context, keys []string, ds pipeline.Signal) error {
	for _, key := range keys {
		c.Add(ctx, key, ds)
	}
	return nil
}

func (c InMemoryKeyCache) Debug(ctx context.Context) {
	c.logger.Debug("IN MEMORY KEY CACHE DEBUG")
	c.logger.Debug("TRACES", zap.Strings("keys", c.tracesFingerprintCache.Keys()))
	c.logger.Debug("METRICS", zap.Strings("keys", c.metricsFingerprintCache.Keys()))
	c.logger.Debug("LOGS", zap.Strings("keys", c.logsFingerprintCache.Keys()))
}

type RedisKeyCache struct {
	redisClient *redis.Client
	tracesTTL   time.Duration
	metricsTTL  time.Duration
	logsTTL     time.Duration

	tenantID string

	logger *zap.Logger
}

type RedisKeyCacheOptions struct {
	Addr                       string
	Username                   string
	Password                   string
	DB                         int
	TracesFingerprintCacheTTL  time.Duration
	MetricsFingerprintCacheTTL time.Duration
	LogsFingerprintCacheTTL    time.Duration
	TenantID                   string
	Logger                     *zap.Logger
}

func NewRedisKeyCache(opts RedisKeyCacheOptions) RedisKeyCache {
	return RedisKeyCache{
		redisClient: redis.NewClient(&redis.Options{
			Addr:     opts.Addr,
			Username: opts.Username,
			Password: opts.Password,
			DB:       opts.DB,
		}),
		tenantID:   opts.TenantID,
		tracesTTL:  opts.TracesFingerprintCacheTTL,
		metricsTTL: opts.MetricsFingerprintCacheTTL,
		logsTTL:    opts.LogsFingerprintCacheTTL,
		logger:     opts.Logger,
	}
}

// getTTL returns the TTL for the given signal type
func (c RedisKeyCache) getTTL(ds pipeline.Signal) time.Duration {
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

func (c RedisKeyCache) Add(ctx context.Context, key string, ds pipeline.Signal) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	redisKey := getKey(c.tenantID, ds)
	ttl := c.getTTL(ds)

	pipe := c.redisClient.Pipeline()
	pipe.SAdd(ctx, redisKey, key)
	pipe.Expire(ctx, redisKey, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (c RedisKeyCache) AddMulti(ctx context.Context, keys []string, ds pipeline.Signal) error {
	if len(keys) == 0 {
		return nil
	}

	for _, key := range keys {
		if key == "" {
			return errors.New("keys cannot be empty")
		}
	}

	redisKey := getKey(c.tenantID, ds)
	ttl := c.getTTL(ds)

	members := make([]interface{}, len(keys))
	for i, key := range keys {
		members[i] = key
	}

	pipe := c.redisClient.Pipeline()
	pipe.SAdd(ctx, redisKey, members...)
	pipe.Expire(ctx, redisKey, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (c RedisKeyCache) Exists(ctx context.Context, key string, ds pipeline.Signal) (bool, error) {
	if key == "" {
		return false, errors.New("key cannot be empty")
	}

	redisKey := getKey(c.tenantID, ds)
	return c.redisClient.SIsMember(ctx, redisKey, key).Result()
}

func (c RedisKeyCache) ExistsMulti(ctx context.Context, keys []string, ds pipeline.Signal) ([]bool, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	members := make([]interface{}, len(keys))
	for i, key := range keys {
		members[i] = key
	}

	redisKey := getKey(c.tenantID, ds)
	results, err := c.redisClient.SMIsMember(ctx, redisKey, members...).Result()
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (c RedisKeyCache) Debug(ctx context.Context) {
	c.logger.Debug("DEBUGGING REDIS KEY CACHE")
	// get the keys matching the patter tenant_id:metadata:* and their cardinality
	keys, err := c.redisClient.Keys(ctx, fmt.Sprintf("%s:metadata:*", c.tenantID)).Result()
	if err != nil {
		c.logger.Error("failed to get keys", zap.Error(err))
	}
	c.logger.Debug("keys", zap.Strings("keys", keys))
	for _, key := range keys {
		cardinality, err := c.redisClient.SCard(ctx, key).Result()
		if err != nil {
			c.logger.Error("failed to get cardinality", zap.Error(err))
		}
		c.logger.Debug("cardinality", zap.String("key", key), zap.Int64("cardinality", cardinality))
	}
}
