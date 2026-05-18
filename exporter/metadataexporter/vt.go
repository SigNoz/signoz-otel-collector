package metadataexporter

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jellydator/ttlcache/v3"
)

type ValueTracker struct {
	ttl             *ttlcache.Cache[string, *lru.Cache[any, struct{}]]
	maxValuesPerKey int
}

func NewValueTracker(
	maxKeys int,
	maxValuesPerKey int,
	ttl time.Duration,
) *ValueTracker {
	cache := ttlcache.New(
		ttlcache.WithTTL[string, *lru.Cache[any, struct{}]](ttl),
		ttlcache.WithCapacity[string, *lru.Cache[any, struct{}]](uint64(maxKeys)),
	)

	go cache.Start()

	return &ValueTracker{
		ttl:             cache,
		maxValuesPerKey: maxValuesPerKey,
	}
}

func (vt *ValueTracker) AddValue(key string, value any) {
	// Get or create LRU cache for this key
	var valueCache *lru.Cache[any, struct{}]
	if item := vt.ttl.Get(key); item != nil {
		valueCache = item.Value()
	} else {
		// Create new LRU cache for this key
		newCache, err := lru.New[any, struct{}](vt.maxValuesPerKey)
		if err != nil {
			return
		}
		valueCache = newCache
		vt.ttl.Set(key, valueCache, ttlcache.DefaultTTL)
	}

	// Add value to the LRU cache
	valueCache.Add(value, struct{}{})
}

func (vt *ValueTracker) GetUniqueValueCount(key string) int {
	if item := vt.ttl.Get(key); item != nil {
		return item.Value().Len()
	}
	return 0
}

func (vt *ValueTracker) Close() {
	vt.ttl.Stop()
}
