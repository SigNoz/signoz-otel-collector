package entity

import (
	"context"

	"go.opentelemetry.io/collector/client"
)

var _ client.AuthData = (*Auth)(nil)

// Auth struct represents the AuthData interface supported by the opentelemetry client.
// It's a domain aggregate.
type Auth struct {
	key    *Key
	tenant *Tenant
}

func NewAuth(key *Key, tenant *Tenant) *Auth {
	return &Auth{
		key:    key,
		tenant: tenant,
	}
}

func (a *Auth) GetAttribute(name string) any {
	switch name {
	case "key.id":
		return a.key.Id
	case "tenant.id":
		return a.tenant.Id
	case "tenant.name":
		return a.tenant.Name
	default:
		return nil
	}
}

func (*Auth) GetAttributeNames() []string {
	return []string{"keyid", "tenantid", "tenantname"}
}

type AuthRepository interface {
	SelectByKeyValue(ctx context.Context, keyValue string) (*Auth, error)
}
