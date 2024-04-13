package off

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

type key struct{}

func newKey() entity.KeyRepository {
	return &key{}
}

func (dao *key) Insert(ctx context.Context, key *entity.Key) error {
	return errors.New(errors.TypeUnsupported, "not supported for strategy off")
}
