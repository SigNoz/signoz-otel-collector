// Brought in as is from opentelemetry-collector-contrib
package regex

import (
	"path/filepath"
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestParserGoldenConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "cache",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Cache.Size = 50
					return cfg
				}(),
			},
			{
				Name: "parse_from_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseFrom = signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("from")}
					return cfg
				}(),
			},
			{
				Name: "parse_to_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("log")}}
					return cfg
				}(),
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
				Name: "timestamp",
				Expect: func() *Config {
					cfg := NewConfig()
					parseField := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("timestamp_field")}
					newTime := signozstanzahelper.TimeParser{
						LayoutType: "strptime",
						Layout:     "%Y-%m-%d",
						ParseFrom:  &parseField,
					}
					cfg.TimeParser = &newTime
					return cfg
				}(),
			},
			{
				Name: "severity",
				Expect: func() *Config {
					cfg := NewConfig()
					parseField := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("severity_field")}
					severityParser := signozstanzahelper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					mapping := map[string]any{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					severityParser.Mapping = mapping
					cfg.SeverityConfig = &severityParser
					return cfg
				}(),
			},
			{
				Name: "regex",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Regex = "^Host=(?P<host>[^,]+), Type=(?P<type>.*)$"
					return cfg
				}(),
			},
			{
				Name: "scope_name",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Regex = "^Host=(?P<host>[^,]+), Logger=(?P<logger_name_field>.*)$"
					parseField := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("logger_name_field")}
					loggerNameParser := signozstanzahelper.NewScopeNameParser()
					loggerNameParser.ParseFrom = parseField
					cfg.ScopeNameParser = &loggerNameParser
					return cfg
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{FieldInterface: entry.NewAttributeField()}}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField()}}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{FieldInterface: entry.NewResourceField()}}
					return p
				}(),
			},
		},
	}.Run(t)
}
