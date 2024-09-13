// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/json"

import (
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

const operatorType = "json_parser"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new JSON parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a JSON parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		ParserOperator: parserOperator,
		json:           jsoniter.ConfigFastest,
	}, nil
}
