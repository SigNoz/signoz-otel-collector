// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"go.opentelemetry.io/collector/component"

	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "normalize"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new normalize config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: signozstanzahelper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a JSON parser operator.
type Config struct {
	signozstanzahelper.TransformerConfig `mapstructure:",squash"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Processor{
		TransformerOperator: transformerOperator,
	}, nil
}
