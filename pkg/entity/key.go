package entity

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"
)

// Key represents an authentication key. It is a domain entity.
// Each key belongs to one tenant.
type Key struct {
	Id        Id
	Name      string
	Value     string
	CreatedAt time.Time
	ExpiresAt time.Time
	TenantId  Id
}

func NewKey(name string, expiresAt time.Time, tenantId Id) *Key {
	return &Key{
		Id:        GenerateId(),
		Name:      name,
		Value:     newKeyValue(),
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		TenantId:  tenantId,
	}
}

// Generates a new random key value.
// The key value is a string consisting of alphanumeric characters (0-9, A-Z, a-z) and hyphens (-)
// and a fixed length of 32 characters
func newKeyValue() string {
	const possibilities = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, 32)
	for i := 0; i < 32; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(possibilities))))
		if err != nil {
			panic(err)
		}
		ret[i] = possibilities[num.Int64()]
	}

	return string(ret)
}

type KeyRepository interface {
	Insert(context.Context, *Key) error
	// Retrieve a key by value. This implies that values should be unique in the database.
	// TODO: Add unique index to key table.
	SelectByValue(context.Context, string) (*Key, error)
}
