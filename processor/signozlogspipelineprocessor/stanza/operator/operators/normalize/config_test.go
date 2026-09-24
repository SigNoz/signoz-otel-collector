package json

import (
	"path/filepath"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
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
				Name: "configured_field_names",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MessageFields = []string{"event", "msg"}
					cfg.SeverityNumberFields = []string{"sev_num"}
					cfg.SeverityTextFields = []string{"sev", "level"}
					cfg.TraceIDFields = []string{"correlation.trace"}
					cfg.SpanIDFields = []string{"correlation.span"}
					cfg.TraceFlagsFields = []string{"correlation.flags"}
					cfg.ScopeNameFields = []string{"component"}
					cfg.ScopeVersionFields = []string{"component_version"}
					return cfg
				}(),
			},
			{
				Name: "empty_field_names",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MessageFields = []string{}
					cfg.SeverityTextFields = []string{}
					return cfg
				}(),
			},
			{
				Name: "move_all_fields",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MoveAllFields = true
					return cfg
				}(),
			},
			{
				Name: "moved_fields",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MoveSeverityNumberField = true
					cfg.MoveSeverityTextField = true
					cfg.MoveTraceIDField = true
					cfg.MoveSpanIDField = true
					cfg.MoveTraceFlagsField = true
					cfg.MoveScopeNameField = true
					cfg.MoveScopeVersionField = true
					return cfg
				}(),
			},
			{
				Name: "move_severity_text_field_only",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MoveSeverityTextField = true
					return cfg
				}(),
			},
		},
	}.Run(t)
}
