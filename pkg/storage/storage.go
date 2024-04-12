package storage

type Storage struct {
	Strategy Strategy
	DAO      DAO
}

func NewStorage(strategy Strategy, opts ...Option) *Storage {
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
