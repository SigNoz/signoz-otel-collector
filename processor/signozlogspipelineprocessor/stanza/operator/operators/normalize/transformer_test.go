package json

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newProcessor(t *testing.T, jsonBodyDualIngestion bool) *Processor {
	cfg := NewConfig()
	cfg.OutputIDs = []string{"fake"}
	cfg.OnError = "drop"
	cfg.JSONBodyDualIngestion = jsonBodyDualIngestion
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	return op.(*Processor)
}

type testCase struct {
	name             string
	expectErr        bool
	input            func() *entry.Entry
	output           func() *entry.Entry
	expectedOriginal any
	originalAsJSONOf map[string]any
}

func TestNormalize(t *testing.T) {
	processor := newProcessor(t, false)

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
			name:     "message_missing_promotes_non_string_compatible_field",
			input:    map[string]any{"msg": 123, "log": 456},
			expected: map[string]any{"message": 456, "msg": 123},
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
				"message":    36,
			},
		},
		{
			name: "message_as_map_flattens_to_top_level_and_message_is_removed",
			input: map[string]any{
				"message": map[string]any{"nested_key": "nested_val", "foo": "bar"},
				"level":   "info",
			},
			expected: map[string]any{
				"nested_key": "nested_val",
				"foo":        "bar",
				"level":      "info",
			},
		},
		{
			name: "message_as_map_flattens_to_top_level_and_message_is_again_map",
			input: map[string]any{
				"message": map[string]any{"nested_key": "nested_val", "foo": "bar", "message": map[string]any{"deep": "value"}},
				"level":   "info",
			},
			expected: map[string]any{
				"nested_key": "nested_val",
				"foo":        "bar",
				"level":      "info",
				"message":    map[string]any{"deep": "value"},
			},
		},
		{
			name:     "message_as_nil_handled_message_is_removed",
			input:    map[string]any{"message": nil, "level": "info"},
			expected: map[string]any{"level": "info"},
		},
		{
			name:     "message_missing_compatible_field_as_map_flattens_after_promotion",
			input:    map[string]any{"msg": map[string]any{"nested_key": "nested_val", "foo": "bar"}, "level": "info"},
			expected: map[string]any{"nested_key": "nested_val", "foo": "bar", "level": "info"},
		},
		{
			name:     "message_as_slice_skipped",
			input:    map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
			expected: map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
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
			originalAsJSONOf: map[string]any{
				"msg":   "test message",
				"level": "info",
				"other": "data",
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
			expectedOriginal: `{"msg": "test message", "level": "info"}`,
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
			expectedOriginal: "Hello World",
		},
	}

	for _, tc := range cases {
		t.Run("Transform/"+tc.name, func(t *testing.T) {
			processor := newProcessor(t, true)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, processor.SetOutputs([]operator.Operator{fake}))

			val := tc.input()
			err := processor.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				requireStashedOriginal(t, val, tc.expectedOriginal, tc.originalAsJSONOf)
				delete(val.Attributes, constants.OriginalBodyAttributeKey)
				if len(val.Attributes) == 0 {
					val.Attributes = nil
				}
				require.Equal(t, tc.output().Body, val.Body)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}

func requireStashedOriginal(t *testing.T, e *entry.Entry, expectedOriginal any, originalAsJSONOf map[string]any) {
	t.Helper()
	original, exists := e.Attributes[constants.OriginalBodyAttributeKey]
	if expectedOriginal == nil && originalAsJSONOf == nil {
		require.False(t, exists)
		return
	}
	require.True(t, exists)
	if originalAsJSONOf != nil {
		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(original.(string)), &parsed))
		require.Equal(t, originalAsJSONOf, parsed)
	} else {
		require.Equal(t, expectedOriginal, original)
	}
}

func TestJSONBodyDualIngestionDefaultsToFalse(t *testing.T) {
	require.False(t, NewConfig().JSONBodyDualIngestion)
}

func TestStashOriginalBodyWhenDualIngestion(t *testing.T) {
	cases := []struct {
		name             string
		input            any
		expectedBody     map[string]any
		expectedOriginal any
		originalAsJSONOf map[string]any
	}{
		{
			name:             "text_body_stashed_as_is",
			input:            "Hello World",
			expectedBody:     map[string]any{"message": "Hello World"},
			expectedOriginal: "Hello World",
		},
		{
			name:             "json_string_body_stashed_byte_exact",
			input:            `{"msg": "hi",   "level": "info"}`,
			expectedBody:     map[string]any{"message": "hi", "level": "info"},
			expectedOriginal: `{"msg": "hi",   "level": "info"}`,
		},
		{
			name:             "scalar_body_stashed_as_is",
			input:            int64(42),
			expectedBody:     map[string]any{"message": int64(42)},
			expectedOriginal: int64(42),
		},
		{
			name:             "nil_body_stashed_as_empty_string",
			input:            nil,
			expectedBody:     map[string]any{},
			expectedOriginal: "",
		},
		{
			name:             "unmutated_map_body_stashed_as_serialized_original",
			input:            map[string]any{"message": "x", "level": "info"},
			expectedBody:     map[string]any{"message": "x", "level": "info"},
			originalAsJSONOf: map[string]any{"message": "x", "level": "info"},
		},
		{
			name:             "map_body_with_msg_promotion_stashed_as_serialized_original",
			input:            map[string]any{"msg": "x", "level": "info"},
			expectedBody:     map[string]any{"message": "x", "level": "info"},
			originalAsJSONOf: map[string]any{"msg": "x", "level": "info"},
		},
		{
			name:             "map_body_with_nil_message_stashed_as_serialized_original",
			input:            map[string]any{"message": nil, "level": "info"},
			expectedBody:     map[string]any{"level": "info"},
			originalAsJSONOf: map[string]any{"message": nil, "level": "info"},
		},
		{
			name:             "map_body_with_message_map_hoist_stashed_as_serialized_original",
			input:            map[string]any{"message": map[string]any{"a": "b"}, "level": "info"},
			expectedBody:     map[string]any{"a": "b", "level": "info"},
			originalAsJSONOf: map[string]any{"message": map[string]any{"a": "b"}, "level": "info"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			processor := newProcessor(t, true)
			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.input
			require.NoError(t, processor.transform(e))
			require.Equal(t, tc.expectedBody, e.Body)
			requireStashedOriginal(t, e, tc.expectedOriginal, tc.originalAsJSONOf)
		})
	}
}

func TestNoStashWhenDualIngestionDisabled(t *testing.T) {
	processor := newProcessor(t, false)
	for _, body := range []any{"Hello World", map[string]any{"msg": "x"}, int64(7)} {
		e := newTestEntryWithTime(t, time.Now())
		e.Body = body
		require.NoError(t, processor.transform(e))
		_, exists := e.Attributes[constants.OriginalBodyAttributeKey]
		require.False(t, exists)
	}
}
