package json

import (
	"encoding/hex"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/stretchr/testify/require"
)

const (
	testTraceID = "6ec8ee2fbde4b8b9dbca5e63a2b41e2f"
	testSpanID  = "a1b2c3d4e5f60718"
)

func decodeHex(t *testing.T, encoded string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(encoded)
	require.NoError(t, err)
	return decoded
}

func TestSetTraceContext(t *testing.T) {
	cases := []struct {
		name            string
		body            any
		attributes      map[string]any
		resource        map[string]any
		traceID         []byte
		spanID          []byte
		expectedTraceID string
		expectedSpanID  string
	}{
		{
			name:            "ids_in_body",
			body:            map[string]any{"message": "boom", "trace_id": testTraceID, "span_id": testSpanID},
			expectedTraceID: testTraceID,
			expectedSpanID:  testSpanID,
		},
		{
			name:            "field_names_are_case_insensitive",
			body:            map[string]any{"traceId": testTraceID, "spanId": testSpanID},
			expectedTraceID: testTraceID,
			expectedSpanID:  testSpanID,
		},
		{
			name:            "flattened_ecs_ids",
			body:            map[string]any{"trace.id": testTraceID, "span.id": testSpanID},
			expectedTraceID: testTraceID,
			expectedSpanID:  testSpanID,
		},
		{
			name:            "ids_in_attributes",
			body:            map[string]any{"message": "boom"},
			attributes:      map[string]any{"trace_id": testTraceID, "span_id": testSpanID},
			expectedTraceID: testTraceID,
			expectedSpanID:  testSpanID,
		},
		{
			name:            "ids_in_resource",
			body:            map[string]any{"message": "boom"},
			resource:        map[string]any{"trace_id": testTraceID},
			expectedTraceID: testTraceID,
		},
		{
			name:            "body_wins_over_attributes",
			body:            map[string]any{"trace_id": testTraceID},
			attributes:      map[string]any{"trace_id": "00000000000000000000000000000001"},
			expectedTraceID: testTraceID,
		},
		{
			name:            "already_decoded_id_is_taken_as_is",
			body:            map[string]any{"message": "boom"},
			attributes:      map[string]any{"trace_id": decodeHexOrPanic(testTraceID)},
			expectedTraceID: testTraceID,
		},
		{
			name: "id_of_the_wrong_length_is_ignored",
			body: map[string]any{"trace_id": "6ec8ee2fbde4b8b9", "span_id": testTraceID},
		},
		{
			name: "id_that_is_not_hex_is_ignored",
			body: map[string]any{"trace_id": "not-a-trace-id-not-a-trace-id-42"},
		},
		{
			name: "all_zero_id_is_ignored",
			body: map[string]any{"trace_id": "00000000000000000000000000000000", "span_id": "0000000000000000"},
		},
		{
			name: "non_string_id_is_ignored",
			body: map[string]any{"trace_id": int64(42)},
		},
		{
			name:            "falls_back_to_next_field_when_value_is_unusable",
			body:            map[string]any{"trace_id": "nope", "trace.id": testTraceID},
			expectedTraceID: testTraceID,
		},
		{
			name:            "existing_ids_are_kept",
			body:            map[string]any{"trace_id": testTraceID, "span_id": testSpanID},
			traceID:         decodeHexOrPanic("11111111111111111111111111111111"),
			spanID:          decodeHexOrPanic("2222222222222222"),
			expectedTraceID: "11111111111111111111111111111111",
			expectedSpanID:  "2222222222222222",
		},
		{
			name: "non_map_body_is_left_alone",
			body: "trace_id=" + testTraceID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry.New()
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource
			e.TraceID = tc.traceID
			e.SpanID = tc.spanID

			defaultFields.infer(e)

			var expectedTraceID, expectedSpanID []byte
			if tc.expectedTraceID != "" {
				expectedTraceID = decodeHex(t, tc.expectedTraceID)
			}
			if tc.expectedSpanID != "" {
				expectedSpanID = decodeHex(t, tc.expectedSpanID)
			}
			require.Equal(t, expectedTraceID, e.TraceID)
			require.Equal(t, expectedSpanID, e.SpanID)
		})
	}
}

func TestSetTraceFlags(t *testing.T) {
	cases := []struct {
		name          string
		body          any
		attributes    map[string]any
		flags         []byte
		expectedFlags []byte
	}{
		{
			name:          "hex_string_in_body",
			body:          map[string]any{"message": "boom", "trace_flags": "01"},
			expectedFlags: []byte{1},
		},
		{
			name:          "field_names_are_case_insensitive",
			body:          map[string]any{"traceFlags": "01"},
			expectedFlags: []byte{1},
		},
		{
			name:          "flattened_ecs_name",
			body:          map[string]any{"trace.flags": "ff"},
			expectedFlags: []byte{255},
		},
		{
			name:          "number_is_read_as_the_flag_bits",
			body:          map[string]any{"trace_flags": int64(1)},
			expectedFlags: []byte{1},
		},
		{
			name:          "not_sampled_is_a_usable_value",
			body:          map[string]any{"trace_flags": "00"},
			expectedFlags: []byte{0},
		},
		{
			name:          "flags_in_attributes",
			body:          map[string]any{"message": "boom"},
			attributes:    map[string]any{"trace_flags": "01"},
			expectedFlags: []byte{1},
		},
		{
			name: "number_out_of_range_is_ignored",
			body: map[string]any{"trace_flags": int64(256)},
		},
		{
			name: "fractional_number_is_ignored",
			body: map[string]any{"trace_flags": 1.5},
		},
		{
			name: "hex_of_the_wrong_length_is_ignored",
			body: map[string]any{"trace_flags": "0001"},
		},
		{
			name: "value_that_is_not_hex_is_ignored",
			body: map[string]any{"trace_flags": "on"},
		},
		{
			name:          "falls_back_to_next_field_when_value_is_unusable",
			body:          map[string]any{"trace_flags": "nope", "trace.flags": "01"},
			expectedFlags: []byte{1},
		},
		{
			name:          "existing_flags_are_kept",
			body:          map[string]any{"trace_flags": "01"},
			flags:         []byte{0},
			expectedFlags: []byte{0},
		},
		{
			name: "non_map_body_is_left_alone",
			body: "trace_flags=01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry.New()
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.TraceFlags = tc.flags

			defaultFields.infer(e)

			require.Equal(t, tc.expectedFlags, e.TraceFlags)
		})
	}
}

func decodeHexOrPanic(encoded string) []byte {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		panic(err)
	}
	return decoded
}
