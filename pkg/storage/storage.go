package storage

import "github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies"

type Storage struct {
	Strategy strategies.Strategy
	DAO      DAO
}

func NewStorage(strategy strategies.Strategy, opts ...Option) *Storage {
	//Set default values
	storageOtps := options{}

	// Merge default and input values
	for _, opt := range opts {
		opt(&storageOtps)
	}

	// Create the dao
	dao := NewDAO(strategy, storageOtps)

	return &Storage{
		Strategy: strategy,
		DAO:      dao,
	}
}
