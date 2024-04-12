package entity

import (
	"crypto/rand"
	"math/big"
	"time"
)

type Key struct {
	Id        Id
	Name      string
	Value     string
	CreatedAt time.Time
	ExpiresAt time.Time
	TenantId  Id
}

func NewKeyValue() string {
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

func NewKey(name string, expiresAt time.Time, tenantId Id) *Key {
	return &Key{
		Id:        NewId(),
		Name:      name,
		Value:     NewKeyValue(),
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		TenantId:  tenantId,
	}
}

type KeyRepository interface {
	Get() []*Key
	GetById(Id) *Key
}
