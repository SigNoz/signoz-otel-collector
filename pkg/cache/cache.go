package cache

import "github.com/SigNoz/signoz-otel-collector/pkg/cache/strategies"

type Cache struct {
	CAO      CAO
	Strategy strategies.Strategy
}

func NewCache(strategy strategies.Strategy, opts ...Option) *Cache {
	cacheOpts := options{}

	// Merge default and input values
	for _, opt := range opts {
		opt(&cacheOpts)
	}

	// Create the cao
	cao := NewCAO(cacheOpts)

	return &Cache{
		CAO:      cao,
		Strategy: strategy,
	}
}
