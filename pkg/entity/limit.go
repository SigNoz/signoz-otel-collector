package entity

import (
	"context"
	"time"
)

// A domain entity representing a limit created by a tenant/consumer.
type Limit struct {
	Id        Id
	Signal    Signal
	Config    LimitConfig
	KeyId     Id
	CreatedAt time.Time
}

func NewLimit(config LimitConfig, keyId Id, signal Signal) *Limit {
	return &Limit{
		Id:        GenerateId(),
		Signal:    signal,
		Config:    config,
		KeyId:     keyId,
		CreatedAt: time.Now(),
	}
}

type LimitRepository interface {
	Insert(context.Context, *Limit) error
	// Selects a limit by key id and signal. Id and Signal should be unique in the database.
	// TODO: Create a unique index on id and signal in the limits table.
	SelectByKeyAndSignal(context.Context, Id, Signal) (*Limit, error)
}
