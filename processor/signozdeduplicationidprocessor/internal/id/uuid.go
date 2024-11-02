package id

import (
	"context"

	"github.com/google/uuid"
)

type uuidGenerator struct{}

func NewUUIDGenerator() Generator {
	return &uuidGenerator{}
}

func (id *uuidGenerator) MustGenerate(_ context.Context) string {
	return uuid.NewString()
}
