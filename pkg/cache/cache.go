package cache

type Cache struct {
	CAO CAO
}

func NewCache(opts ...Option) *Cache {
	cacheOpts := options{}

	// Merge default and input values
	for _, opt := range opts {
		opt(&cacheOpts)
	}

	// Create the cao
	cao := NewCAO(cacheOpts)

	return &Cache{
		CAO: cao,
	}
}
