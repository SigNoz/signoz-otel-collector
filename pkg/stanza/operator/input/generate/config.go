// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/input/generate"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

const operatorType = "generate_input"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new generate input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a generate input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	Entry              entry.Entry `mapstructure:"entry"`
	Count              int         `mapstructure:"count"`
	Static             bool        `mapstructure:"static"`
}

// Build will build a generate input operator.
func (c *Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	c.Entry.Body = recursiveMapInterfaceToMapString(c.Entry.Body)

	return &Input{
		InputOperator: inputOperator,
		entry:         c.Entry,
		count:         c.Count,
		static:        c.Static,
	}, nil
}
