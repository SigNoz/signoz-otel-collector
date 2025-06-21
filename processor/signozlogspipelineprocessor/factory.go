// Brought in as is from logstransform processor in opentelemetry-collector-contrib
// with identifiers changed for the new processor
package signozlogspipelineprocessor

import (
	"context"
	"errors"
	"fmt"

	signozlogspipelinestanzaadapter "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/adapter"
	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability))
}

// Note: This isn't a valid configuration (no operators would lead to no work being done)
func createDefaultConfig() component.Config {
	return &Config{
		BaseConfig: signozlogspipelinestanzaadapter.BaseConfig{
			Operators: []signozlogspipelinestanzaoperator.Config{},
		},
	}
}

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize signozlogspipeline processor")
	}
	if len(pCfg.BaseConfig.Operators) == 0 {
		return nil, errors.New("no operators were configured for signozlogspipeline processor")
	}

	proc, err := newLogsPipelineProcessor(pCfg, set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("couldn't build \"signozlogspipeline\" processor %w", err)
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithStart(proc.Start),
		processorhelper.WithShutdown(proc.Shutdown),
		processorhelper.WithCapabilities(processorCapabilities),
	)
}
