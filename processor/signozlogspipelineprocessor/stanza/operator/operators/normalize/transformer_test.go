package json

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newProcessor(t *testing.T) *Processor {
	cfg := NewConfig()
	cfg.OutputIDs = []string{"fake"}
	cfg.OnError = "drop"
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	return op.(*Processor)
}

type testCase struct {
	name      string
	expectErr bool
	input     func() *entry.Entry
	output    func() *entry.Entry
}

func TestNormalize(t *testing.T) {
	processor := newProcessor(t)

	cases := []struct {
		name     string
		input    any
		expected map[string]any
	}{
		{
			name:     "message_already_exists_as_string",
			input:    map[string]any{"message": "hello", "level": "info"},
			expected: map[string]any{"message": "hello", "level": "info"},
		},
		{
			name:     "message_missing_msg_field_moved_to_message",
			input:    map[string]any{"msg": "test message", "level": "info"},
			expected: map[string]any{"message": "test message", "level": "info"},
		},
		{
			name:     "message_present_log_field_also_present",
			input:    map[string]any{"message": "message content", "log": "log content", "other": "data"},
			expected: map[string]any{"message": "message content", "log": "log content", "other": "data"},
		},
		{
			name:     "message_missing_prefers_log_over_msg_when_both_present",
			input:    map[string]any{"log": "from log", "msg": "from msg"},
			expected: map[string]any{"message": "from log", "msg": "from msg"},
		},
		{
			name:     "message_missing_skips_non_string_msg_compatible_fields",
			input:    map[string]any{"msg": 123, "log": 456},
			expected: map[string]any{"msg": 123, "log": 456},
		},
		{
			name:     "message_missing_no_compatible_fields",
			input:    map[string]any{"level": "info", "other": "data"},
			expected: map[string]any{"level": "info", "other": "data"},
		},
		{
			name: "message_as_map_flattens_to_top_level",
			input: map[string]any{
				"message": map[string]any{"nested_key": "nested_val", "foo": "bar", "message": 36},
				"level":   "info",
			},
			expected: map[string]any{
				"nested_key": "nested_val",
				"foo":        "bar",
				"level":      "info",
				"message":    "36",
			},
		},
		{
			name:     "message_as_slice_marshals_to_json_string",
			input:    map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
			expected: map[string]any{"message": `["a","b","c"]`, "level": "info"},
		},
		{
			name:     "message_as_bool_converted_to_string",
			input:    map[string]any{"message": true, "level": "info"},
			expected: map[string]any{"message": "true", "level": "info"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.input
			processor.normalize(e)
			require.Equal(t, tc.expected, e.Body)
		})
	}
}

func newTestEntryWithTime(_ *testing.T, now time.Time) *entry.Entry {
	e := entry.New()
	e.ObservedTimestamp = now
	e.Timestamp = time.Unix(1586632809, 0)
	return e
}

func TestTransform(t *testing.T) {
	now := time.Now()

	cases := []testCase{
		{
			name:      "moves_msg_field_to_message_and_deletes_original",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"msg":   "test message",
					"level": "info",
					"other": "data",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"level":   "info",
					"other":   "data",
					"message": "test message",
				}
				return e
			},
		},
		{
			name:      "json_string_with_msg_field_parses_and_restructures",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = `{"msg": "test message", "level": "info"}`
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"message": "test message",
					"level":   "info",
				}
				return e
			},
		},
		{
			name:      "text_logs_transformed_to_json",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = `Hello World`
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"message": "Hello World",
				}
				return e
			},
		},
	}

	for _, tc := range cases {
		t.Run("Transform/"+tc.name, func(t *testing.T) {
			processor := newProcessor(t)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, processor.SetOutputs([]operator.Operator{fake}))

			val := tc.input()
			err := processor.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.output().Body, val.Body)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
