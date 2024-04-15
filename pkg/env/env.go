package env

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/cache"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
)

// This variable is used to store the global state of the process.
// Think of it as global dependency injection. This is required to inject external dependencies into the
// collector process
var state g

// Access the global environment
func G() g {
	return state
}

type g struct {
	storage *storage.Storage
	cache   *cache.Cache
}

func NewG(opts ...Option) {
	var g g
	// Merge default and input values
	for _, opt := range opts {
		opt(&g)
	}

	state = g
}

func (env g) Storage() *storage.Storage {
	return env.storage
}

func (env g) Cache() *cache.Cache {
	return env.cache
}
