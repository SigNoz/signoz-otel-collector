// Brought in as is from opentelemetry-collector-contrib
package copy

import (
	"path/filepath"
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "body_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("key")}
					cfg.To = entry.NewBodyField("key2")
					return cfg
				}(),
			},
			{
				Name: "body_to_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("key")}
					cfg.To = entry.NewAttributeField("key2")
					return cfg
				}(),
			},
			{
				Name: "attribute_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("key")}
					cfg.To = entry.NewResourceField("key2")
					return cfg
				}(),
			},
			{
				Name: "attribute_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("key")}
					cfg.To = entry.NewBodyField("key2")
					return cfg
				}(),
			},
			{
				Name: "attribute_to_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("key")}
					cfg.To = entry.NewAttributeField("one", "two", "three")
					return cfg
				}(),
			},
			{
				Name: "resource_to_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = signozstanzaentry.Field{FieldInterface: entry.NewResourceField("key")}
					cfg.To = entry.NewResourceField("one", "two", "three")
					return cfg
				}(),
			},
		},
	}.Run(t)
}
