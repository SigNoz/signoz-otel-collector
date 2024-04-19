package entity

import (
	"context"

	"go.opentelemetry.io/collector/client"
)

var _ client.AuthData = (*Auth)(nil)

const (
	MetadataKeyId      = "signoz-key-id"
	MetadataTenantName = "signoz-tenant-name"
)

// Auth struct represents the AuthData interface like the opentelemetry client.
// We cannot use the otel client because some processors like the batch processor overwrite
// the entire context. See code below extracted from the batch processor:
//
//	exportCtx := client.NewContext(context.Background(), client.Info{
//		Metadata: client.NewMetadata(md),
//	})
//
// The only workaround to a new client metadata and use that. Ensure that the metadata_keys in the
// batch processor match the keys set here.
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

// NewMetadat takes an existing metadat and derives a new metadata with the
// Auth value stored on it.
func NewMetadataWithAuth(ctx context.Context, auth *Auth) client.Metadata {
	return client.NewMetadata(map[string][]string{
		MetadataKeyId:      {auth.key.Id.String()},
		MetadataTenantName: {auth.tenant.Name},
	})
}

func KeyIdFromMetadata(md client.Metadata) (Id, bool) {
	values := md.Get(MetadataKeyId)

	if len(values) == 0 {
		return Id{}, false
	}

	return MustNewId(values[0]), true
}

func TenantNameFromMetadata(md client.Metadata) (string, bool) {
	values := md.Get(MetadataKeyId)

	if len(values) == 0 {
		return "", false
	}

	return values[0], true
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
