// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unquote // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/transformer/unquote"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

const operatorType = "unquote"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new unquote config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new unquote config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of an unquote operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
}

// Build will build an unquote operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		field:               c.Field,
	}, nil
}
