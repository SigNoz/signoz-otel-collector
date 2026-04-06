package signozllmcostprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmcostprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

const typeStr = "signozllmcost"

// NewFactory returns the component factory for signozllmcostprocessor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
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
