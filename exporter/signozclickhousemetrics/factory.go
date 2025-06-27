package signozclickhousemetrics

import (
	"context"
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
)

// NewFactory creates a new ClickHouse Metrics exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		internalmetadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, internalmetadata.MetricsStability))
}

func createMetricsExporter(ctx context.Context, set exporter.Settings,
	cfg component.Config) (exporter.Metrics, error) {

	chCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	connOptions, err := clickhouse.ParseDSN(chCfg.DSN)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, err
	}

	cache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](45*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)

	chExporter, err := NewClickHouseExporter(
		WithConfig(chCfg),
		WithConn(conn),
		WithLogger(set.Logger),
		WithMeter(set.MeterProvider.Meter(internalmetadata.ScopeName)),
		WithEnableExpHist(chCfg.EnableExpHist),
		WithSettings(set),
		WithCache(cache, chCfg.DisableTtlCache),
	)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		chExporter.PushMetrics,
		exporterhelper.WithTimeout(chCfg.TimeoutConfig),
		exporterhelper.WithQueue(chCfg.QueueBatchConfig),
		exporterhelper.WithRetry(chCfg.BackOffConfig),
		exporterhelper.WithStart(chExporter.Start),
		exporterhelper.WithShutdown(chExporter.Shutdown),
	)

	if err != nil {
		return nil, err
	}

	return exporter, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		QueueBatchConfig: exporterhelper.NewDefaultQueueConfig(),
		DSN:              "tcp://localhost:9000",
		EnableExpHist:    false,
		Database:         "signoz_metrics",
		SamplesTable:     "distributed_samples_v4",
		TimeSeriesTable:  "distributed_time_series_v4",
		ExpHistTable:     "distributed_exp_hist",
		MetadataTable:    "distributed_metadata",
		DisableTtlCache: false,
	}
}
