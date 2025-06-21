// Brought in as is from logstransform processor in opentelemetry-collector-contrib
package signozlogspipelineprocessor

import (
	"context"
	"testing"

	signozlogspipelinestanzaadapter "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/adapter"
	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		BaseConfig: signozlogspipelinestanzaadapter.BaseConfig{
			Operators: []signozlogspipelinestanzaoperator.Config{
				{
					Builder: func() *regex.Config {
						cfg := regex.NewConfig()
						cfg.Regex = "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"
						sevField := entry.NewAttributeField("sev")
						sevCfg := helper.NewSeverityConfig()
						sevCfg.ParseFrom = &sevField
						cfg.SeverityConfig = &sevCfg
						timeField := entry.NewAttributeField("time")
						timeCfg := helper.NewTimeParser()
						timeCfg.Layout = "%Y-%m-%d"
						timeCfg.ParseFrom = &timeField
						cfg.TimeParser = &timeCfg
						return cfg
					}(),
				},
			},
		},
	}

	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
}

func TestInvalidOperators(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		BaseConfig: signozlogspipelinestanzaadapter.BaseConfig{
			Operators: []signozlogspipelinestanzaoperator.Config{
				{
					// invalid due to missing regex
					Builder: regex.NewConfig(),
				},
			},
		},
	}

	_, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)
}
