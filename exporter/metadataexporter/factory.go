package metadataexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
)

const (
	DefaultMaxResources              = 8192
	DefaultMaxCardinalityPerResource = 2048
	DefaultMaxTotalCardinality       = 3_000_000

	// DefaultTimeout bounds one INSERT into the metadata table; it is also
	// applied as max_execution_time on the ClickHouse connection.
	DefaultTimeout = 30 * time.Second

	// DefaultBucket is the write window for traces and logs sets.
	DefaultBucket = 6 * time.Hour
	// DefaultMetricsBucket is the write window for metrics sets. It matches
	// the six-hour floor the fields API applies to a request start; a longer
	// bucket (metric sets are long-lived) needs the reader to floor the start
	// to it first.
	DefaultMetricsBucket = 6 * time.Hour
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
		TimeoutConfig:    exporterhelper.TimeoutConfig{Timeout: DefaultTimeout},
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		DSN:              "tcp://localhost:9000",
		MaxDistinctValues: MaxDistinctValuesConfig{
			Traces: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxResourceStringLength: 64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
				Bucket:                  DefaultBucket,
			},
			Logs: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxResourceStringLength: 64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
				Bucket:                  DefaultBucket,
			},
			Metrics: LimitsConfig{
				MaxKeys:                 4096,
				MaxStringLength:         64,
				MaxResourceStringLength: 64,
				MaxStringDistinctValues: 2048,
				FetchInterval:           15 * time.Minute,
				Bucket:                  DefaultMetricsBucket,
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
		JSON: JSONConfig{
			Enabled:                 false,
			MaxDepthTraverse:        to.Ptr(defaultJSONMaxDepthTraverse),
			MaxArrayElementsAllowed: to.Ptr(defaultJSONMaxArrayElementsAllowed),
			MaxKeysAtLevel:          to.Ptr(defaultJSONMaxKeysAtLevel),
		},
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
	exp, err := newMetadataExporter(ctx, oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(
		ctx,
		set,
		&oCfg,
		exp.PushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
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
	exp, err := newMetadataExporter(ctx, oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetrics(
		ctx,
		set,
		&oCfg,
		exp.PushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
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
	exp, err := newMetadataExporter(ctx, oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(
		ctx,
		set,
		&oCfg,
		exp.PushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutConfig),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueBatchConfig),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown))
}
