package off

import (
	"context"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

type limit struct{}

func newLimit() entity.LimitRepository {
	return &limit{}
}

func (dao *limit) Insert(ctx context.Context, limit *entity.Limit) error {
	return errors.New(errors.TypeUnsupported, "not supported for strategy off")
}

func (dao *limit) SelectByKeyAndSignal(ctx context.Context, keyId entity.Id, signal entity.Signal) (*entity.Limit, error) {
	return nil, errors.New(errors.TypeUnsupported, "not supported for strategy off")
}

type limitMetric struct{}

func newLimitMetric() entity.LimitMetricRepository {
	return &limitMetric{}
}

func (dao *limitMetric) Insert(ctx context.Context, limitMetrics []*entity.LimitMetric) error {
	return errors.New(errors.TypeUnsupported, "not supported for strategy off")
}

func (dao *limitMetric) SelectValueByPeriodAndSignalAndPeriodAtAndKeyId(ctx context.Context, period entity.Period, signal entity.Signal, periodAt time.Time, keyId entity.Id) (entity.LimitValue, error) {
	return entity.LimitValue{}, errors.New(errors.TypeUnsupported, "not supported for strategy off")
}
