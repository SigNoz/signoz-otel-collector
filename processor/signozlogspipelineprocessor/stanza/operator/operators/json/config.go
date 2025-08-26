// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"go.opentelemetry.io/collector/component"

	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "json_parser"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new JSON parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: signozstanzahelper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a JSON parser operator.
type Config struct {
	signozstanzahelper.ParserConfig `mapstructure:",squash"`
	EnableFlattening                bool   `mapstructure:"enable_flattening"`
	MaxFlatteningDepth              int    `mapstructure:"max_flattening_depth"`
	EnablePaths                     bool   `mapstructure:"enable_paths"`
	PathPrefix                      string `mapstructure:"path_prefix"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.MaxFlatteningDepth < 0 {
		c.MaxFlatteningDepth = 0
	}

	return &Parser{
		ParserOperator:     parserOperator,
		enableFlattening:   c.EnableFlattening,
		maxFlatteningDepth: c.MaxFlatteningDepth,
		enablePaths:        c.EnablePaths,
		pathPrefix:         c.PathPrefix,
	}, nil
}
