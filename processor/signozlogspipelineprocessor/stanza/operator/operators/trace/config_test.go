// Brought in as is from opentelemetry-collector-contrib
package trace

import (
	"path/filepath"
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
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
				Name: "spanid",
				Expect: func() *Config {
					parseFrom := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("app_span_id")}
					cfg := signozstanzahelper.SpanIDConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.SpanID = &cfg
					return c
				}(),
			},
			{
				Name: "traceid",
				Expect: func() *Config {
					parseFrom := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("app_trace_id")}
					cfg := signozstanzahelper.TraceIDConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.TraceID = &cfg
					return c
				}(),
			},
			{
				Name: "trace_flags",
				Expect: func() *Config {
					parseFrom := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("app_trace_flags_id")}
					cfg := signozstanzahelper.TraceFlagsConfig{}
					cfg.ParseFrom = &parseFrom

					c := NewConfig()
					c.TraceFlags = &cfg
					return c
				}(),
			},
		},
	}.Run(t)
}
