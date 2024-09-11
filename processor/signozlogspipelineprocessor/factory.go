// Brought in as is from signozlogspipeline processor in opentelemetry-collector-contrib
// with identifiers changed for the new processor
package signozlogspipelineprocessor

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType("signozlogspipeline"),
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, component.StabilityLevelDevelopment))
}

// Note: This isn't a valid configuration (no operators would lead to no work being done)
func createDefaultConfig() component.Config {
	return &Config{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
	}
}

func createLogsProcessor(
	_ context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize signozlogspipeline processor")
	}

	if len(pCfg.BaseConfig.Operators) == 0 {
		return nil, errors.New("no operators were configured for this signozlogspipeline processor")
	}

	return newProcessor(pCfg, nextConsumer, set.TelemetrySettings)
}
