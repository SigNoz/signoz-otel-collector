package limiter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.Type("limiter"),
		createDefaultConfig,
		processor.WithTraces(createTraces, component.StabilityLevelDevelopment),
		processor.WithMetrics(createMetrics, component.StabilityLevelDevelopment),
		processor.WithLogs(createLogs, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTraces(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	lp, err := newLimiterProcessor(set.Logger)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		lp.ConsumeTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		processorhelper.WithStart(lp.Start),
		processorhelper.WithShutdown(lp.Shutdown),
	)
}

func createMetrics(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	lp, err := newLimiterProcessor(set.Logger)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		lp.ConsumeMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		processorhelper.WithStart(lp.Start),
		processorhelper.WithShutdown(lp.Shutdown),
	)
}

func createLogs(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	lp, err := newLimiterProcessor(set.Logger)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		lp.ConsumeLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		processorhelper.WithStart(lp.Start),
		processorhelper.WithShutdown(lp.Shutdown),
	)
}
