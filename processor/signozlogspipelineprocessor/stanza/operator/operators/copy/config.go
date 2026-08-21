// Brought in as is from opentelemetry-collector-contrib

package copy

import (
	"go.opentelemetry.io/collector/component"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "copy"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new copy operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new copy operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: signozstanzahelper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a copy operator
type Config struct {
	signozstanzahelper.TransformerConfig `mapstructure:",squash"`
	From                                 signozstanzaentry.Field `mapstructure:"from"`
	To                                   entry.Field             `mapstructure:"to"`
}

// Build will build a copy operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	// Upstream errors here; SigNoz warns to keep accepting configs that started before upstream changes.
	if c.From == (signozstanzaentry.Field{}) {
		set.Logger.Warn("copy: missing 'from' field; this operator is misconfigured. Set 'from' to silence this warning.")
	}

	if c.To.IsEmpty() {
		set.Logger.Warn("copy: missing 'to' field; this operator is misconfigured. Set 'to' to silence this warning.")
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		From:                c.From,
		To:                  c.To,
	}, nil
}
