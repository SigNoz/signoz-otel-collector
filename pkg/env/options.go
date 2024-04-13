package env

import "github.com/SigNoz/signoz-otel-collector/pkg/storage"

type Option func(*g)

// WithStorage returns the storage struct for the given strategy.
func WithStorage(storage *storage.Storage) Option {
	return func(o *g) {
		o.storage = storage
	}
}
