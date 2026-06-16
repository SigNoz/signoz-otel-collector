// Async processor: ConsumeLogs returns immediately after enqueuing to
// FromPdataConverter. The factory therefore bypasses processorhelper.NewLogs
// (which only supports sync ProcessLogs callbacks) and returns the processor
// directly as a processor.Logs implementation.
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

func createLogsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize signozlogspipeline processor")
	}
	if len(pCfg.Operators) == 0 {
		return nil, errors.New("no operators were configured for signozlogspipeline processor")
	}

	proc, err := newLogsPipelineProcessor(pCfg, set, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("couldn't build \"signozlogspipeline\" processor %w", err)
	}

	return proc, nil
}
