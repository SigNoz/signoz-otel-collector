package off

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

type tenant struct{}

func newTenant() entity.TenantRepository {
	return &tenant{}
}

func (dao *tenant) Insert(ctx context.Context, tenant *entity.Tenant) error {
	return errors.New(errors.TypeUnsupported, "not supported for strategy off")
}

func (dao *tenant) SelectByName(ctx context.Context, name string) (*entity.Tenant, error) {
	return nil, errors.New(errors.TypeUnsupported, "not supported for strategy off")
}
