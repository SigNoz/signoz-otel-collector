package limiter

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/env"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type limiterProcessor struct {
	logger      *zap.Logger
	storage     *storage.Storage
	policy      entity.LimitMetricRepository
	shutdownCh  chan bool
	incrementCh chan []*entity.LimitMetric
	goroutines  sync.WaitGroup
}

func newLimiterProcessor(logger *zap.Logger, cfg component.Config) (*limiterProcessor, error) {
	config, _ := cfg.(*Config)
	var policy entity.LimitMetricRepository

	if config.Policy == PolicyRedis {
		policy = NewRedisPolicy(env.G().Cache())
	} else {
		policy = NewPostgresPolicy(env.G().Storage())
	}

	return &limiterProcessor{
		logger:      logger,
		storage:     env.G().Storage(),
		policy:      policy,
		shutdownCh:  make(chan bool),
		incrementCh: make(chan []*entity.LimitMetric, runtime.NumCPU()),
		goroutines:  sync.WaitGroup{},
	}, nil
}

func (lp *limiterProcessor) Start(ctx context.Context, h component.Host) error {
	lp.goroutines.Add(1)
	go func() {
	MAIN:
		for {
			select {
			case limitMetrics := <-lp.incrementCh:
				lp.increment(ctx, limitMetrics)

			case <-lp.shutdownCh:
				lp.logger.Info("Flushing remaining metrics", zap.Int("num", len(lp.incrementCh)))

			FLUSH:
				for {
					select {
					case limitMetrics := <-lp.incrementCh:
						lp.increment(ctx, limitMetrics)
					default:
						break FLUSH
					}
				}

				lp.logger.Info("Stopping increment goroutine")
				break MAIN
			}
		}

		lp.goroutines.Done()
	}()
	return nil
}

func (lp *limiterProcessor) Shutdown(ctx context.Context) error {
	// Signal the increment loop to close
	lp.shutdownCh <- true

	// Wait for the increment loop to exit
	lp.goroutines.Wait()
	return nil
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

func (lp *limiterProcessor) increment(ctx context.Context, limitMetrics []*entity.LimitMetric) {
	err := lp.policy.Insert(ctx, limitMetrics)
	if err != nil {
		lp.logger.Error("unable to increment limit metrics", zap.Error(err))
	}
}

func (lp *limiterProcessor) consume(ctx context.Context, signal entity.Signal, limitValue entity.LimitValue) error {
	// Retrieve credential id from ctx
	auth, ok := entity.AuthFromContext(ctx)
	if !ok {
		// If no auth was found, skip the limiter.
		return nil
	}

	keyId := auth.KeyId()

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
		currLimitValue, err := lp.policy.
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

	// Send the limitMetric to be ingested asynchronously
	lp.incrementCh <- limitMetrics

	return nil
}
