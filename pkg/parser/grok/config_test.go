// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package grok

import (
	"path/filepath"
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
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
					cfg.ParseFrom = signozstanzaentry.Field{signozstanzaentry.NewBodyField("from")}
					return cfg
				}(),
			},
			{
				Name: "parse_to_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{signozstanzaentry.NewBodyField("log")}}
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
					parseField := signozstanzaentry.Field{signozstanzaentry.NewBodyField("timestamp_field")}
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
					parseField := signozstanzaentry.Field{signozstanzaentry.NewBodyField("severity_field")}
					severityParser := signozstanzahelper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					mapping := map[string]interface{}{
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
				Name: "pattern",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Pattern = "a=%{NOTSPACE:data}"
					return cfg
				}(),
			},
			{
				Name: "scope_name",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Pattern = "a=%{NOTSPACE:data}"
					parseField := signozstanzaentry.Field{signozstanzaentry.NewBodyField("logger_name_field")}
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
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{entry.NewAttributeField()}}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{signozstanzaentry.NewBodyField()}}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = signozstanzaentry.RootableField{Field: signozstanzaentry.Field{entry.NewResourceField()}}
					return p
				}(),
			},
		},
	}.Run(t)
}
