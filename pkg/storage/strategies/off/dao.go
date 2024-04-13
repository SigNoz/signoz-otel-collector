package off

import "github.com/SigNoz/signoz-otel-collector/pkg/entity"

type off struct{}

func NewDAO() *off {
	return &off{}
}

func (dao *off) Tenants() entity.TenantRepository {
	return newTenant()
}

func (dao *off) Keys() entity.KeyRepository {
	return newKey()
}
