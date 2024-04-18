package off

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

type auth struct{}

func newAuth() entity.AuthRepository {
	return &auth{}
}

func (dao *auth) SelectByKeyValue(ctx context.Context, keyValue string) (*entity.Auth, error) {
	return nil, errors.New(errors.TypeUnsupported, "not supported for strategy off")
}
