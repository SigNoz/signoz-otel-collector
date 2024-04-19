package entity

import (
	"context"

	"go.opentelemetry.io/collector/client"
)

var _ client.AuthData = (*Auth)(nil)

// unexported context key
type authCtxKey struct{}

// Auth struct represents the AuthData interface like the opentelemetry client.
// We cannot use the otel client because some processors like the batch processor overwrite
// the entire context. See code below extracted from the batch processor:
//
//	exportCtx := client.NewContext(context.Background(), client.Info{
//		Metadata: client.NewMetadata(md),
//	})
//
// # Use the FromContext and NewContext functions directly instead of relying on AuthData
//
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

func NewContextWithAuth(ctx context.Context, auth *Auth) context.Context {
	return context.WithValue(ctx, authCtxKey{}, auth)
}

func AuthFromContext(ctx context.Context) (*Auth, bool) {
	c, ok := ctx.Value(authCtxKey{}).(*Auth)
	return c, ok
}

func (a *Auth) KeyId() Id {
	return a.key.Id
}

func (a *Auth) TenantName() string {
	return a.tenant.Name
}

// CAUTION: Do not use this function unless you are sure no processor overwrites the context
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

// CAUTION: Do not use this function unless you are sure no processor overwrites the context
func (*Auth) GetAttributeNames() []string {
	return []string{"keyid", "tenantid", "tenantname"}
}

type AuthRepository interface {
	SelectByKeyValue(ctx context.Context, keyValue string) (*Auth, error)
}
