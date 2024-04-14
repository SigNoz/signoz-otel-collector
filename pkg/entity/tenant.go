package entity

import (
	"context"
	"time"
)

// Tenant represents an tenant entity.
// These can be thought of as the consumers of this
// distribution.
type Tenant struct {
	Id        Id
	Name      string
	CreatedAt time.Time
}

func NewTenant(name string) *Tenant {
	return &Tenant{
		Id:        GenerateId(),
		Name:      name,
		CreatedAt: time.Now(),
	}
}

type TenantRepository interface {
	Insert(context.Context, *Tenant) error
	// Retrieve a tenant by name. This implies that names should be unique in the database.
	// TODO: Add unique index on name to tenant table.
	SelectByName(context.Context, string) (*Tenant, error)
}
