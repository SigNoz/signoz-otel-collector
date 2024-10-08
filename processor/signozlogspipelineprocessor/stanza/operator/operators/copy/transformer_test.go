// Brought in as is from opentelemetry-collector-contrib
package copy

import (
	"context"
	"testing"
	"time"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type testCase struct {
	name      string
	expectErr bool
	op        *Config
	input     func() *entry.Entry
	output    func() *entry.Entry
}

// Test building and processing a Config
func TestBuildAndProcess(t *testing.T) {
	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		e.Body = map[string]any{
			"key": "val",
			"nested": map[string]any{
				"nestedkey": "nestedval",
			},
		}
		return e
	}

	cases := []testCase{
		{
			"body_to_body",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"key2": "val",
				}
				return e
			},
		},
		{
			"nested_to_body",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("nested", "nestedkey")}
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"key2": "nestedval",
				}
				return e
			},
		},
		{
			"body_to_nested",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewBodyField("nested", "key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
						"key2":      "val",
					},
				}
				return e
			},
		},
		{
			"body_to_attribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewAttributeField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				e.Attributes = map[string]any{"key2": "val"}
				return e
			},
		},
		{
			"body_to_nested_attribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField()}
				cfg.To = entry.NewAttributeField("one", "two")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "val",
							"nested": map[string]any{
								"nestedkey": "nestedval",
							},
						},
					},
				}
				return e
			},
		},
		{
			"body_to_nested_resource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField()}
				cfg.To = entry.NewResourceField("one", "two")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "val",
							"nested": map[string]any{
								"nestedkey": "nestedval",
							},
						},
					},
				}
				return e
			},
		},
		{
			"attribute_to_body",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{entry.NewAttributeField("key")}
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{"key": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"key2": "val",
				}
				e.Attributes = map[string]any{"key": "val"}
				return e
			},
		},
		{
			"attribute_to_resource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{entry.NewAttributeField("key")}
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{"key": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{"key": "val"}
				e.Resource = map[string]any{"key2": "val"}
				return e
			},
		},
		{
			"overwrite",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewBodyField("nested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key":    "val",
					"nested": "val",
				}
				return e
			},
		},
		{
			"invalid_copy_to_attribute_root",
			true,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewAttributeField()
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_copy_to_resource_root",
			true,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{signozstanzaentry.NewBodyField("key")}
				cfg.To = entry.NewResourceField()
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_key",
			true,
			func() *Config {
				cfg := NewConfig()
				cfg.From = signozstanzaentry.Field{entry.NewAttributeField("nonexistentkey")}
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
			newTestEntry,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run("BuildAndProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			cp := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, cp.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = cp.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
