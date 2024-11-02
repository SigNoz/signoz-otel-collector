package signozdeduplicationidprocessor

import (
	"context"
	"sync"

	"github.com/SigNoz/signoz-otel-collector/processor/signozdeduplicationidprocessor/internal/metadata"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type factory struct {
	// restrict only 1 limiter with a policy to be active.
	// Eg: If postgres is active, redis cant be useed.
	deduplicationIds map[component.Config]*deduplicationIdProcessor
	lock             sync.Mutex
}

func NewFactory() processor.Factory {
	f := &factory{
		deduplicationIds: make(map[component.Config]*deduplicationIdProcessor),
	}
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Generator: "uuid",
	}
}

func (f *factory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	deduplicationId, err := f.getDeduplicationId(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		deduplicationId.ConsumeTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(deduplicationId.Start),
		processorhelper.WithShutdown(deduplicationId.Shutdown),
	)
}

func (f *factory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	deduplicationId, err := f.getDeduplicationId(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		deduplicationId.ConsumeMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(deduplicationId.Start),
		processorhelper.WithShutdown(deduplicationId.Shutdown),
	)
}

func (f *factory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	deduplicationId, err := f.getDeduplicationId(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		deduplicationId.ConsumeLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(deduplicationId.Start),
		processorhelper.WithShutdown(deduplicationId.Shutdown),
	)
}

func (f *factory) getDeduplicationId(set processor.Settings, cfg component.Config) (*deduplicationIdProcessor, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if deduplicationId, ok := f.deduplicationIds[cfg]; ok {
		return deduplicationId, nil
	}

	deduplicationId, err := newDeduplicationIdProcessor(set, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	f.deduplicationIds[cfg] = deduplicationId
	return deduplicationId, nil
}
