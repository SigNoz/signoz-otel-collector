package entity

import (
	"context"
	"time"
)

type Tenant struct {
	Id        Id
	Name      string
	CreatedAt time.Time
}

func NewTenant(name string) *Tenant {
	return &Tenant{
		Id:        NewId(),
		Name:      name,
		CreatedAt: time.Now(),
	}
}

type TenantRepository interface {
	Insert(context.Context, *Tenant) error
	SelectByName(context.Context, string) (*Tenant, error)
}
