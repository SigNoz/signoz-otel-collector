package json

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newProcessor(t *testing.T) *Processor {
	cfg := newInferenceConfig()
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
			name:     "message_missing_body_field_moved_to_message",
			input:    map[string]any{"body": "test message", "level": "info"},
			expected: map[string]any{"message": "test message", "level": "info"},
		},
		{
			name:     "message_missing_prefers_body_over_log_and_msg",
			input:    map[string]any{"body": "from body", "log": "from log", "msg": "from msg"},
			expected: map[string]any{"message": "from body", "log": "from log", "msg": "from msg"},
		},
		{
			name:     "message_missing_differently_cased_message_moved_to_message",
			input:    map[string]any{"Message": "hello", "level": "info"},
			expected: map[string]any{"message": "hello", "level": "info"},
		},
		{
			name:     "message_missing_differently_cased_compatible_field_moved_to_message",
			input:    map[string]any{"MSG": "hello", "level": "info"},
			expected: map[string]any{"message": "hello", "level": "info"},
		},
		{
			name:     "message_missing_prefers_differently_cased_message_over_compatible_fields",
			input:    map[string]any{"Message": "from message", "msg": "from msg"},
			expected: map[string]any{"message": "from message", "msg": "from msg"},
		},
		{
			name:     "exact_message_wins_over_differently_cased_one",
			input:    map[string]any{"message": "exact", "Message": "cased"},
			expected: map[string]any{"message": "exact", "Message": "cased"},
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

func TestNormalizeMessageFromAttributesAndResource(t *testing.T) {
	processor := newProcessor(t)

	cases := []struct {
		name               string
		body               any
		attributes         map[string]any
		resource           map[string]any
		expectedBody       map[string]any
		expectedAttributes map[string]any
		expectedResource   map[string]any
	}{
		{
			name:               "message_in_attributes",
			body:               map[string]any{"level": "info"},
			attributes:         map[string]any{"message": "hello"},
			expectedBody:       map[string]any{"level": "info", "message": "hello"},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "message_compatible_field_in_attributes",
			body:               map[string]any{"level": "info"},
			attributes:         map[string]any{"msg": "hello", "other": "data"},
			expectedBody:       map[string]any{"level": "info", "message": "hello"},
			expectedAttributes: map[string]any{"other": "data"},
		},
		{
			name:             "message_in_resource",
			body:             map[string]any{"level": "info"},
			resource:         map[string]any{"body": "hello"},
			expectedBody:     map[string]any{"level": "info", "message": "hello"},
			expectedResource: map[string]any{},
		},
		{
			name:               "body_field_wins_over_attributes",
			body:               map[string]any{"msg": "from body"},
			attributes:         map[string]any{"message": "from attributes"},
			expectedBody:       map[string]any{"message": "from body"},
			expectedAttributes: map[string]any{"message": "from attributes"},
		},
		{
			name:               "attributes_win_over_resource",
			body:               map[string]any{"level": "info"},
			attributes:         map[string]any{"msg": "from attributes"},
			resource:           map[string]any{"message": "from resource"},
			expectedBody:       map[string]any{"level": "info", "message": "from attributes"},
			expectedAttributes: map[string]any{},
			expectedResource:   map[string]any{"message": "from resource"},
		},
		{
			name:               "existing_message_is_kept",
			body:               map[string]any{"message": "from body"},
			attributes:         map[string]any{"message": "from attributes"},
			expectedBody:       map[string]any{"message": "from body"},
			expectedAttributes: map[string]any{"message": "from attributes"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource

			processor.normalize(e)

			require.Equal(t, tc.expectedBody, e.Body)
			require.Equal(t, tc.expectedAttributes, e.Attributes)
			require.Equal(t, tc.expectedResource, e.Resource)
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
					"other":   "data",
					"level":   "info",
					"message": "test message",
				}
				e.Severity = entry.Info
				e.SeverityText = "INFO"
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
				e.Severity = entry.Info
				e.SeverityText = "INFO"
				return e
			},
		},
		{
			name:      "otel_fields_carried_in_the_body_are_lifted_onto_the_record",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = `{"body": "checked out", "severity_text": "warning", "trace_id": "` + testTraceID +
					`", "span_id": "` + testSpanID + `", "scope.name": "checkout", "scope.version": "1.4.0"}`
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"message":       "checked out",
					"severity_text": "warning",
					"trace_id":      testTraceID,
					"span_id":       testSpanID,
					"scope.name":    "checkout",
					"scope.version": "1.4.0",
				}
				e.Severity = entry.Warn
				e.SeverityText = "WARN"
				e.TraceID = decodeHexOrPanic(testTraceID)
				e.SpanID = decodeHexOrPanic(testSpanID)
				e.ScopeName = "checkout"
				e.Attributes = map[string]any{
					signozstanzaentry.InternalTempScopeVersionAttribute: "1.4.0",
				}
				return e
			},
		},
		{
			name:      "otel_fields_nested_under_a_message_compatible_field_are_lifted_too",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"log": map[string]any{"level": "error", "message": "boom", "scope_name": "cart"},
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{
					"message":    "boom",
					"level":      "error",
					"scope_name": "cart",
				}
				e.Severity = entry.Error
				e.SeverityText = "ERROR"
				e.ScopeName = "cart"
				return e
			},
		},
		{
			name:      "otel_fields_carried_in_the_attributes_are_lifted_onto_the_record",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = `something failed`
				e.Attributes = map[string]any{"level": "error", "trace_id": testTraceID}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntryWithTime(t, now)
				e.Body = map[string]any{"message": "something failed"}
				e.Attributes = map[string]any{"level": "error", "trace_id": testTraceID}
				e.Severity = entry.Error
				e.SeverityText = "ERROR"
				e.TraceID = decodeHexOrPanic(testTraceID)
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
