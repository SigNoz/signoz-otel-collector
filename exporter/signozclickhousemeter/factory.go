package signozclickhousemeter

import (
	"context"
	"errors"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemeter/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory creates a new ClickHouse Meter Metrics exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		internalmetadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMeterMetricsExporter, internalmetadata.MetricsStability))
}

func createMeterMetricsExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	chCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	chExporter, err := NewClickHouseExporter(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetrics(
		ctx,
		params,
		cfg,
		chExporter.ConsumeMetrics,
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
		Database:         "signoz_meter",
		SamplesTable:     "distributed_samples",
	}
}
