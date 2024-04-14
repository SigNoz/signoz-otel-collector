package limiter

import (
	"context"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/env"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type limiterProcessor struct {
	logger  *zap.Logger
	storage *storage.Storage
}

func newLimiterProcessor(logger *zap.Logger) (*limiterProcessor, error) {
	return &limiterProcessor{
		logger:  logger,
		storage: env.G().Storage(),
	}, nil
}

func (lp *limiterProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	err := lp.consume(
		ctx,
		entity.SignalTraces,
		entity.NewLimitValue(
			td.SpanCount(),
			(&ptrace.ProtoMarshaler{}).TracesSize(td),
		),
	)

	if err != nil {
		return td, err
	}

	return td, nil
}

func (lp *limiterProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	err := lp.consume(
		ctx,
		entity.SignalMetrics,
		entity.NewLimitValue(
			md.MetricCount(),
			(&pmetric.ProtoMarshaler{}).MetricsSize(md),
		),
	)

	if err != nil {
		return md, err
	}

	return md, nil
}

func (lp *limiterProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	err := lp.consume(
		ctx,
		entity.SignalLogs,
		entity.NewLimitValue(
			ld.LogRecordCount(),
			(&plog.ProtoMarshaler{}).LogsSize(ld),
		),
	)

	if err != nil {
		return ld, err
	}

	return ld, nil
}

func (lp *limiterProcessor) consume(ctx context.Context, signal entity.Signal, limitValue entity.LimitValue) error {
	// Retrieve credential id from ctx
	keyId, ok := entity.KeyIdFromCtx(ctx)
	if !ok {
		return nil
	}

	// Retrieve the limit from the backing store
	limit, err := lp.storage.DAO.Limits().SelectByKeyAndSignal(ctx, keyId, signal)
	if err != nil {
		t, _, _ := errors.Unwrapb(err)
		if t == errors.TypeNotFound {
			return nil
		}

		return err
	}

	// Set the current time here for rate limiting
	currTime := time.Now()

	// Iterate over all periods and check whether limit has exceeded or not
	for period, maxLimitValue := range limit.Config.Map() {
		currLimitValue, err := lp.storage.DAO.LimitMetrics().
			SelectValueByPeriodAndSignalAndPeriodAtAndKeyId(
				ctx,
				period,
				signal,
				entity.PeriodAt(currTime)[period],
				keyId,
			)
		if err != nil {
			t, _, _ := errors.Unwrapb(err)
			if t != errors.TypeNotFound {
				return err
			}
			// If the current limit metric for the specified period was not found, initialize to zero
			currLimitValue = entity.NewLimitValue(0, 0)
		}

		// Return error on the period that led to the limit
		if maxLimitValue.Subtract(currLimitValue).Subtract(limitValue).LessThan(0) {
			return status.Error(codes.ResourceExhausted, "limit exceeded")
		}
	}

	// Otherwise increment the limit metrics and only create entries for limits specified in the config map
	var limitMetrics []*entity.LimitMetric
	for period := range limit.Config.Map() {
		limitMetrics = append(limitMetrics, entity.NewLimitMetric(
			period,
			limitValue,
			signal,
			entity.PeriodAt(currTime)[period],
			keyId,
		))
	}

	err = lp.storage.DAO.LimitMetrics().Insert(ctx, limitMetrics)
	if err != nil {
		return err
	}

	return nil
}
