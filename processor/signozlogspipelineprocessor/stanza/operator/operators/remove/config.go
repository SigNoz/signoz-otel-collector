// Brought in as is from opentelemetry-collector-contrib

package remove

import (
	"go.opentelemetry.io/collector/component"

	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "remove"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new remove operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new remove operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: signozstanzahelper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a remove operator
type Config struct {
	signozstanzahelper.TransformerConfig `mapstructure:",squash"`

	Field rootableField `mapstructure:"field"`
}

// Build will build a Remove operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	// Upstream errors here; SigNoz warns to keep accepting configs that started before upstream changes
	if c.Field.IsEmpty() {
		set.Logger.Warn("remove: missing 'field'; this operator is misconfigured. Set 'field' to silence this warning.")
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		Field:               c.Field,
	}, nil
}
