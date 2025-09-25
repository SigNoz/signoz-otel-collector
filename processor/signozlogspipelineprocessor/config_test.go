// Brought in as is from logstransform processor in opentelemetry-collector-contrib
package signozlogspipelineprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

	signozlogspipelinestanzaadapter "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/adapter"
	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/regex"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t, &Config{
		BaseConfig: signozlogspipelinestanzaadapter.BaseConfig{
			Operators: []signozlogspipelinestanzaoperator.Config{
				{
					Builder: func() *regex.Config {
						cfg := regex.NewConfig()
						cfg.Regex = "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"
						sevField := signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("sev")}
						sevCfg := signozstanzahelper.NewSeverityConfig()
						sevCfg.ParseFrom = &sevField
						cfg.SeverityConfig = &sevCfg
						timeField := signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("time")}
						timeCfg := signozstanzahelper.NewTimeParser()
						timeCfg.Layout = "%Y-%m-%d %H:%M:%S"
						timeCfg.ParseFrom = &timeField
						cfg.TimeParser = &timeCfg
						return cfg
					}(),
				},
			},
		},
	}, cfg)
}
