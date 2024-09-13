// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package time // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/time"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

const operatorType = "time_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new time parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new time parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		TimeParser:        helper.NewTimeParser(),
	}
}

// Config is the configuration of a time parser operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	helper.TimeParser        `mapstructure:",omitempty,squash"`
}

func (c *Config) Unmarshal(component *confmap.Conf) error {
	return component.Unmarshal(c, confmap.WithIgnoreUnused())
}

// Build will build a time parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if err := c.TimeParser.Validate(); err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		TimeParser:          c.TimeParser,
	}, nil
}
