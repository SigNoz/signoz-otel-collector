package signozmemorylimiterprocessor

import (
	"context"
	"sync"

	"github.com/SigNoz/signoz-otel-collector/processor/signozmemorylimiterprocessor/internal/metadata"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var processorCapabilities = consumer.Capabilities{MutatesData: false}

type factory struct {
	// restrict only 1 limiter with a policy to be active.
	// Eg: If postgres is active, redis cant be useed.
	memoryLimiters map[component.Config]*memoryLimiterProcessor
	lock           sync.Mutex
}

func NewFactory() processor.Factory {
	f := &factory{
		memoryLimiters: make(map[component.Config]*memoryLimiterProcessor),
	}
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
	)
}

// CreateDefaultConfig creates the default configuration for processor. Notice
// that the default configuration is expected to fail for this processor.
func createDefaultConfig() component.Config {
	return &Config{}
}

func (f *factory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	memoryLimiter, err := f.getMemoryLimiter(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		memoryLimiter.ConsumeTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(memoryLimiter.Start),
		processorhelper.WithShutdown(memoryLimiter.Shutdown),
	)
}

func (f *factory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	memoryLimiter, err := f.getMemoryLimiter(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		memoryLimiter.ConsumeMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(memoryLimiter.Start),
		processorhelper.WithShutdown(memoryLimiter.Shutdown),
	)
}

func (f *factory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	memoryLimiter, err := f.getMemoryLimiter(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		memoryLimiter.ConsumeLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(memoryLimiter.Start),
		processorhelper.WithShutdown(memoryLimiter.Shutdown),
	)
}

func (f *factory) getMemoryLimiter(set processor.Settings, cfg component.Config) (*memoryLimiterProcessor, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if memoryLimiter, ok := f.memoryLimiters[cfg]; ok {
		return memoryLimiter, nil
	}

	memoryLimiter, err := newMemoryLimiterProcessor(set, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	f.memoryLimiters[cfg] = memoryLimiter
	return memoryLimiter, nil
}
