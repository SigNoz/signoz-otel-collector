package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver/internal/metadata"
)

// systemTablesMetricsReceiver snapshots system.view_refreshes on an interval and
// emits the current state as OTel gauge metrics.
type systemTablesMetricsReceiver struct {
	config     *Config
	metricsCfg *SystemTablesMetricsConfig

	mb *metadata.MetricsBuilder

	clickhouse   clickhouseQuerier
	nextConsumer consumer.Metrics

	logger  *zap.Logger
	obsrecv *receiverhelper.ObsReport

	requestShutdown    context.CancelFunc
	shutdownCompleteWg sync.WaitGroup
}

func (r *systemTablesMetricsReceiver) Start(_ context.Context, _ component.Host) error {
	receiverCtx, cancel := context.WithCancel(context.Background())
	r.requestShutdown = cancel
	r.shutdownCompleteWg.Add(1)
	go r.run(receiverCtx)
	return nil
}

func (r *systemTablesMetricsReceiver) Shutdown(context.Context) error {
	if r.requestShutdown != nil {
		r.requestShutdown()
		r.shutdownCompleteWg.Wait()
	}
	return nil
}

func (r *systemTablesMetricsReceiver) run(ctx context.Context) {
	defer r.shutdownCompleteWg.Done()

	ticker := time.NewTicker(r.metricsCfg.CollectionInterval)
	defer ticker.Stop()

	r.logger.Info(
		"starting clickhouse system.view_refreshes metrics scrape",
		zap.Duration("collection_interval", r.metricsCfg.CollectionInterval),
	)

	if err := r.scrape(ctx); err != nil {
		r.logger.Error("metrics scrape attempt failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopping metrics receiver run loop")
			return
		case <-ticker.C:
			if err := r.scrape(ctx); err != nil {
				r.logger.Error("metrics scrape attempt failed", zap.Error(err))
			}
		}
	}
}

func (r *systemTablesMetricsReceiver) scrape(ctx context.Context) error {
	if r.clickhouse == nil {
		db, err := newClickhouseClient(ctx, r.config.DSN)
		if err != nil {
			return fmt.Errorf("couldn't create clickhouse client: %w", err)
		}
		r.clickhouse = newClickhouseQuerrier(db, r.config.ClusterName)
	}

	ctx = r.obsrecv.StartMetricsOp(ctx)

	metrics, err := r.collect(ctx)
	if err != nil {
		r.obsrecv.EndMetricsOp(ctx, "clickhouse.system_tables", 0, err)
		return err
	}

	if metrics.DataPointCount() == 0 {
		r.obsrecv.EndMetricsOp(ctx, "clickhouse.system_tables", 0, nil)
		return nil
	}

	consumeErr := r.nextConsumer.ConsumeMetrics(ctx, metrics)
	r.obsrecv.EndMetricsOp(ctx, "clickhouse.system_tables", metrics.DataPointCount(), consumeErr)
	if consumeErr != nil {
		return fmt.Errorf("couldn't push metrics to next consumer: %w", consumeErr)
	}
	return nil
}

// collect records view_refreshes rows grouped by replica hostname.
func (r *systemTablesMetricsReceiver) collect(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	views, err := r.clickhouse.scrapeViewRefreshes(ctx)
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("couldn't scrape system.view_refreshes: %w", err)
	}

	viewsByHost := map[string][]ViewRefreshRow{}
	for _, v := range views {
		viewsByHost[v.Hostname] = append(viewsByHost[v.Hostname], v)
	}

	for hostname, views := range viewsByHost {
		for _, v := range views {
			// LastSuccessAge is -1 when the view has never succeeded; skip the age
			// datapoint so it surfaces as missing data.
			if v.LastSuccessAge >= 0 {
				r.mb.RecordClickhouseViewRefreshLastSuccessAgeDataPoint(now, v.LastSuccessAge, v.Database, v.View)
			}
			r.mb.RecordClickhouseViewRefreshLastDurationDataPoint(now, v.LastDuration, v.Database, v.View)
			r.mb.RecordClickhouseViewRefreshExceptionDataPoint(now, int64(v.Exception), v.Database, v.View)
			r.mb.RecordClickhouseViewRefreshRetryDataPoint(now, v.Retry, v.Database, v.View)
			r.mb.RecordClickhouseViewRefreshProgressDataPoint(now, v.Progress, v.Database, v.View)
		}

		rb := r.mb.NewResourceBuilder()
		rb.SetClickhouseHostname(hostname)
		r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return r.mb.Emit(), nil
}
