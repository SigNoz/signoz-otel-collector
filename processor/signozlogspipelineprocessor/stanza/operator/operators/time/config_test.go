// Brought in as is from opentelemetry-collector-contrib
package time

import (
	"path/filepath"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "parse_strptime",
				Expect: func() *Config {
					cfg := NewConfig()
					from := entry.NewBodyField("from")
					cfg.ParseFrom = &from
					cfg.LayoutType = "strptime"
					cfg.Layout = "%Y-%m-%d"
					return cfg
				}(),
			},
			{
				Name: "parse_gotime",
				Expect: func() *Config {
					cfg := NewConfig()
					from := entry.NewBodyField("from")
					cfg.ParseFrom = &from
					cfg.LayoutType = "gotime"
					cfg.Layout = "2006-01-02"
					return cfg
				}(),
			},
		},
	}.Run(t)
}
