package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/jmoiron/sqlx"
)

type limit struct {
	db *sqlx.DB
}

func newLimit(db *sqlx.DB) entity.LimitRepository {
	return &limit{
		db: db,
	}
}

func (dao *limit) Insert(ctx context.Context, limit *entity.Limit) error {
	_, err := dao.
		db.
		ExecContext(
			ctx,
			"INSERT INTO public.limit (id, signal, config, created_at, key_id) VALUES ($1, $2, $3, $4, $5)",
			limit.Id, limit.Signal, limit.Config, limit.CreatedAt, limit.KeyId,
		)
	if err != nil {
		return err
	}

	return nil
}

func (dao *limit) SelectByKeyAndSignal(ctx context.Context, keyId entity.Id, signal entity.Signal) (*entity.Limit, error) {
	limit := new(entity.Limit)
	err := dao.
		db.
		QueryRowContext(
			ctx,
			"SELECT id, signal, config, created_at, key_id FROM public.limit WHERE key_id = $1 AND signal = $2",
			keyId,
			signal,
		).Scan(&limit.Id, &limit.Signal, &limit.Config, &limit.CreatedAt, &limit.KeyId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Newf(errors.TypeNotFound, "limit not found")
		}
		return nil, err
	}

	return limit, nil
}

type limitMetric struct {
	db *sqlx.DB
}

func newLimitMetric(db *sqlx.DB) entity.LimitMetricRepository {
	return &limitMetric{
		db: db,
	}
}

func (dao *limitMetric) Insert(ctx context.Context, limitMetrics []*entity.LimitMetric) error {
	tx, err := dao.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	for _, limitMetric := range limitMetrics {
		_, err = tx.
			ExecContext(
				ctx,
				"INSERT INTO limit_metric (period, count, size, signal, period_at, key_id) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (period, signal, period_at, key_id) DO UPDATE SET count = limit_metric.count + EXCLUDED.count, size = limit_metric.size + EXCLUDED.size",
				limitMetric.Period, limitMetric.Value.Count, limitMetric.Value.Size, limitMetric.Signal, limitMetric.PeriodAt, limitMetric.KeyId,
			)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (dao *limitMetric) SelectValueByPeriodAndSignalAndPeriodAtAndKeyId(ctx context.Context, period entity.Period, signal entity.Signal, periodAt time.Time, keyId entity.Id) (entity.LimitValue, error) {
	var count int
	var size int

	err := dao.
		db.
		QueryRowContext(
			ctx,
			"SELECT count, size FROM limit_metric WHERE period = $1 AND signal = $2 AND period_at = $3 AND key_id = $4",
			period,
			signal,
			periodAt,
			keyId,
		).Scan(&count, &size)
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.LimitValue{}, errors.Newf(errors.TypeNotFound, "limit not found")
		}
		return entity.LimitValue{}, err
	}

	return entity.NewLimitValue(count, size), nil
}
