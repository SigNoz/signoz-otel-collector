package signozfilterprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/SigNoz/signoz-otel-collector/processor/signozfilterprocessor/internal/metadata"
)

const defaultMaxAllowedDataAgeDays = 15

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{MaxAllowedDataAgeDays: defaultMaxAllowedDataAgeDays}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	proc := newProcessor(cfg.(*Config))
	return processorhelper.NewLogs(
		ctx, set, cfg, nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	proc := newProcessor(cfg.(*Config))
	return processorhelper.NewTraces(
		ctx, set, cfg, nextConsumer,
		proc.ProcessTraces,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}
