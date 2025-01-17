// Brought in as is from opentelemetry-collector-contrib

package regex

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "regex_parser"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new regex parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new regex parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: signozstanzahelper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a regex parser operator.
type Config struct {
	signozstanzahelper.ParserConfig `mapstructure:",squash"`

	Regex string `mapstructure:"regex"`

	Cache struct {
		Size uint16 `mapstructure:"size"`
	} `mapstructure:"cache"`
}

// Build will build a regex parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.Regex == "" {
		return nil, fmt.Errorf("missing required field 'regex'")
	}

	r, err := regexp.Compile(c.Regex)
	if err != nil {
		return nil, fmt.Errorf("compiling regex: %w", err)
	}

	namedCaptureGroups := 0
	for _, groupName := range r.SubexpNames() {
		if groupName != "" {
			namedCaptureGroups++
		}
	}
	if namedCaptureGroups == 0 {
		return nil, errors.NewError(
			"no named capture groups in regex pattern",
			"use named capture groups like '^(?P<my_key>.*)$' to specify the key name for the parsed field",
		)
	}

	op := &Parser{
		ParserOperator: parserOperator,
		regexp:         r,
	}

	if c.Cache.Size > 0 {
		op.cache = newMemoryCache(c.Cache.Size, 0)
		set.Logger.Debug(
			"configured memory cache",
			zap.String("operator_id", op.ID()),
			zap.Uint16("size", op.cache.maxSize()),
		)
	}

	return op, nil
}
