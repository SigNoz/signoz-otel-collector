// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scope // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/scope"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

const operatorType = "scope_name_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new logger name parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new logger name parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		ScopeNameParser:   helper.NewScopeNameParser(),
	}
}

// Config is the configuration of a logger name parser operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	helper.ScopeNameParser   `mapstructure:",omitempty,squash"`
}

// Build will build a logger name parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		ScopeNameParser:     c.ScopeNameParser,
	}, nil
}
