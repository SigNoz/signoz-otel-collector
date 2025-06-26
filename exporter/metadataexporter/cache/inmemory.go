package metadataexporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type resourceEntry struct {
	mu    sync.RWMutex
	attrs map[uint64]struct{} // set of attribute fingerprints
}

type InMemoryKeyCacheOptions struct {
	// Maximum # of resource fingerprints for each pipeline:
	MaxTracesResourceFp  uint64
	MaxMetricsResourceFp uint64
	MaxLogsResourceFp    uint64

	// Maximum # of attribute fingerprints per resource:
	MaxTracesCardinalityPerResource  uint64
	MaxMetricsCardinalityPerResource uint64
	MaxLogsCardinalityPerResource    uint64

	// TTL for each pipeline’s resource entries
	TracesFingerprintCacheTTL  time.Duration
	MetricsFingerprintCacheTTL time.Duration
	LogsFingerprintCacheTTL    time.Duration

	TracesMaxTotalCardinality  uint64
	MetricsMaxTotalCardinality uint64
	LogsMaxTotalCardinality    uint64

	TenantID string
	Logger   *zap.Logger

	Debug bool
}

type InMemoryKeyCache struct {
	tracesCache  *ttlcache.Cache[uint64, *resourceEntry]
	metricsCache *ttlcache.Cache[uint64, *resourceEntry]
	logsCache    *ttlcache.Cache[uint64, *resourceEntry]

	maxTracesResourceFp  uint64
	maxMetricsResourceFp uint64
	maxLogsResourceFp    uint64

	maxTracesCardinalityPerResource  uint64
	maxMetricsCardinalityPerResource uint64
	maxLogsCardinalityPerResource    uint64

	tracesMaxTotalCardinality  uint64
	metricsMaxTotalCardinality uint64
	logsMaxTotalCardinality    uint64

	tenantID string
	logger   *zap.Logger

	debug bool
}

var _ KeyCache = (*InMemoryKeyCache)(nil)

func NewInMemoryKeyCache(opts InMemoryKeyCacheOptions) (*InMemoryKeyCache, error) {
	// Build the TTL caches for each pipeline
	tracesCache := ttlcache.New[uint64, *resourceEntry](
		ttlcache.WithTTL[uint64, *resourceEntry](opts.TracesFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[uint64, *resourceEntry](),
	)
	go tracesCache.Start()

	metricsCache := ttlcache.New[uint64, *resourceEntry](
		ttlcache.WithTTL[uint64, *resourceEntry](opts.MetricsFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[uint64, *resourceEntry](),
	)
	go metricsCache.Start()

	logsCache := ttlcache.New[uint64, *resourceEntry](
		ttlcache.WithTTL[uint64, *resourceEntry](opts.LogsFingerprintCacheTTL),
		ttlcache.WithDisableTouchOnHit[uint64, *resourceEntry](),
	)
	go logsCache.Start()

	return &InMemoryKeyCache{
		tracesCache:  tracesCache,
		metricsCache: metricsCache,
		logsCache:    logsCache,

		maxTracesResourceFp:  opts.MaxTracesResourceFp,
		maxMetricsResourceFp: opts.MaxMetricsResourceFp,
		maxLogsResourceFp:    opts.MaxLogsResourceFp,

		maxTracesCardinalityPerResource:  opts.MaxTracesCardinalityPerResource,
		maxMetricsCardinalityPerResource: opts.MaxMetricsCardinalityPerResource,
		maxLogsCardinalityPerResource:    opts.MaxLogsCardinalityPerResource,

		tracesMaxTotalCardinality:  opts.TracesMaxTotalCardinality,
		metricsMaxTotalCardinality: opts.MetricsMaxTotalCardinality,
		logsMaxTotalCardinality:    opts.LogsMaxTotalCardinality,

		tenantID: opts.TenantID,
		logger:   opts.Logger,
		debug:    opts.Debug,
	}, nil
}

// getCacheAndLimits picks the appropriate TTL cache and cardinalities based on the pipeline signal.
func (c *InMemoryKeyCache) getCacheAndLimits(ds pipeline.Signal) (*ttlcache.Cache[uint64, *resourceEntry], uint64, uint64) {
	switch ds {
	case pipeline.SignalTraces:
		return c.tracesCache, c.maxTracesResourceFp, c.maxTracesCardinalityPerResource
	case pipeline.SignalMetrics:
		return c.metricsCache, c.maxMetricsResourceFp, c.maxMetricsCardinalityPerResource
	case pipeline.SignalLogs:
		return c.logsCache, c.maxLogsResourceFp, c.maxLogsCardinalityPerResource
	}
	return nil, 0, 0
}

// AddAttrsToResource adds attribute fingerprints for the given resourceFp, respecting capacity.
func (c *InMemoryKeyCache) AddAttrsToResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) error {
	cache, maxResourceCount, maxAttrCount := c.getCacheAndLimits(ds)

	// If resource entry doesn’t exist, we may need to create it
	entry := cache.Get(resourceFp)
	if entry == nil {
		// Check how many total resources are in the cache
		if uint64(len(cache.Keys())) >= maxResourceCount {
			return fmt.Errorf("too many resource fingerprints in %s cache", ds.String())
		}
		// Create a new resource entry
		newEntry := &resourceEntry{
			attrs: make(map[uint64]struct{}),
		}
		cache.Set(resourceFp, newEntry, ttlcache.DefaultTTL)
		entry = cache.Get(resourceFp)
	}

	// Now add attributes to that resource
	entry.Value().mu.Lock()
	defer entry.Value().mu.Unlock()

	// Check if adding these attributes will exceed the limit
	if uint64(len(entry.Value().attrs))+uint64(len(attrFps)) > maxAttrCount {
		return fmt.Errorf("too many attribute fingerprints for resource %d in %s cache",
			resourceFp, ds.String())
	}

	for _, a := range attrFps {
		entry.Value().attrs[a] = struct{}{}
	}
	return nil
}

// AttrsExistForResource checks for existence of each attribute in the resource’s set.
func (c *InMemoryKeyCache) AttrsExistForResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) ([]bool, error) {
	cache, _, _ := c.getCacheAndLimits(ds)

	entry := cache.Get(resourceFp)
	if entry == nil {
		// none of them exist
		out := make([]bool, len(attrFps))
		return out, nil
	}

	entry.Value().mu.RLock()
	defer entry.Value().mu.RUnlock()

	out := make([]bool, len(attrFps))
	for i, a := range attrFps {
		_, exists := entry.Value().attrs[a]
		out[i] = exists
	}
	return out, nil
}

func (c *InMemoryKeyCache) ResourcesLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	cache, _, _ := c.getCacheAndLimits(ds)
	return uint64(len(cache.Keys())) >= c.maxTracesResourceFp
}

func (c *InMemoryKeyCache) TotalCardinalityLimitExceeded(ctx context.Context, ds pipeline.Signal) bool {
	// combine the cardinality of all resources
	var totalCardinality uint64
	for _, resourceFp := range c.tracesCache.Keys() {
		entry := c.tracesCache.Get(resourceFp)
		totalCardinality += uint64(len(entry.Value().attrs))
	}
	return totalCardinality >= c.maxTracesCardinalityPerResource
}

func (c *InMemoryKeyCache) CardinalityLimitExceededMulti(ctx context.Context, resourceFps []uint64, ds pipeline.Signal) ([]bool, error) {
	cache, _, _ := c.getCacheAndLimits(ds)

	out := make([]bool, len(resourceFps))
	for i, resourceFp := range resourceFps {
		entry := cache.Get(resourceFp)
		if entry == nil {
			out[i] = false
			continue
		}
		out[i] = uint64(len(entry.Value().attrs)) >= c.maxTracesCardinalityPerResource
	}
	return out, nil
}

func (c *InMemoryKeyCache) CardinalityLimitExceeded(ctx context.Context, resourceFp uint64, ds pipeline.Signal) bool {
	cache, _, _ := c.getCacheAndLimits(ds)
	entry := cache.Get(resourceFp)
	if entry == nil {
		return false
	}
	return uint64(len(entry.Value().attrs)) >= c.maxTracesCardinalityPerResource
}

func (c *InMemoryKeyCache) Debug(ctx context.Context) {
	if !c.debug {
		return
	}

	c.logger.Debug("IN MEMORY KEY CACHE DEBUG")

	dump := func(name string, cache *ttlcache.Cache[uint64, *resourceEntry]) {
		var info []string
		for _, resourceFp := range cache.Keys() {
			e := cache.Get(resourceFp)
			if e == nil {
				continue
			}
			e.Value().mu.RLock()
			size := len(e.Value().attrs)
			e.Value().mu.RUnlock()
			info = append(info, fmt.Sprintf("resFp=%d: #attrs=%d", resourceFp, size))
		}
		c.logger.Debug(name, zap.Strings("resources", info))
	}

	dump("TRACES", c.tracesCache)
	dump("METRICS", c.metricsCache)
	dump("LOGS", c.logsCache)
}

func (c *InMemoryKeyCache) Close(ctx context.Context) error {
	if c.tracesCache != nil {
		c.tracesCache.Stop()
	}
	if c.metricsCache != nil {
		c.metricsCache.Stop()
	}
	if c.logsCache != nil {
		c.logsCache.Stop()
	}
	return nil
}
