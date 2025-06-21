package metadataexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
)

const (
	DefaultMaxResources              = 8192
	DefaultMaxCardinalityPerResource = 2048
	DefaultMaxTotalCardinality       = 3_000_000
)

// NewFactory creates Metadata exporter factory.
func NewFactory() exporter.Factory {
	f := &metadataExporterFactory{}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(f.createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		QueueBatchConfig: exporterhelper.NewDefaultQueueConfig(),
		DSN:              "tcp://localhost:9000",
		MaxDistinctValues: MaxDistinctValuesConfig{
			Traces: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
			},
			Logs: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
			},
			Metrics: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
			},
		},
		Cache: CacheConfig{
			Provider: CacheProviderInMemory,
			InMemory: InMemoryCacheConfig{},
			Traces: CacheLimits{
				MaxResources:              DefaultMaxResources,
				MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
				MaxTotalCardinality:       DefaultMaxTotalCardinality,
			},
			Metrics: CacheLimits{
				MaxResources:              DefaultMaxResources,
				MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
				MaxTotalCardinality:       DefaultMaxTotalCardinality,
			},
			Logs: CacheLimits{
				MaxResources:              DefaultMaxResources,
				MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
				MaxTotalCardinality:       DefaultMaxTotalCardinality,
			},
			Debug: false,
		},
		Enabled: false,
	}
}

type metadataExporterFactory struct {
}

func (f *metadataExporterFactory) createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp, err := newMetadataExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(
		ctx,
		set,
		&oCfg,
		exp.PushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueBatchConfig),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown))
}

func (f *metadataExporterFactory) createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp, err := newMetadataExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetrics(
		ctx,
		set,
		&oCfg,
		exp.PushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueBatchConfig),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown))
}

func (f *metadataExporterFactory) createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp, err := newMetadataExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(
		ctx,
		set,
		&oCfg,
		exp.PushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueBatchConfig),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown))
}
