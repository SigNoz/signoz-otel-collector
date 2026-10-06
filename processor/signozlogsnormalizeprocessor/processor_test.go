package signozlogsnormalizeprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsnormalizeprocessor/internal/metadatatest"
)

func testConfig(dualIngestion bool) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.JSONBodyDualIngestion = dualIngestion
	return cfg
}

func newTestProcessor(t *testing.T, dualIngestion bool) *normalizeProcessor {
	t.Helper()
	p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), testConfig(dualIngestion))
	require.NoError(t, err)
	return p
}

func newLogsWithBodies(t *testing.T, bodies ...any) plog.Logs {
	t.Helper()
	ld := plog.NewLogs()
	lrs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for _, body := range bodies {
		require.NoError(t, lrs.AppendEmpty().Body().FromRaw(body))
	}
	return ld
}

func processSingle(t *testing.T, p *normalizeProcessor, body any) plog.LogRecord {
	t.Helper()
	out, err := p.ProcessLogs(context.Background(), newLogsWithBodies(t, body))
	require.NoError(t, err)
	return out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
}

func requireNoStash(t *testing.T, lr plog.LogRecord) {
	t.Helper()
	_, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
	require.False(t, exists)
}

func TestNormalizeMessage(t *testing.T) {
	cases := []struct {
		name     string
		input    map[string]any
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
			input:    map[string]any{"msg": int64(123), "log": int64(456)},
			expected: map[string]any{"message": int64(456), "msg": int64(123)},
		},
		{
			name:     "message_missing_no_compatible_fields",
			input:    map[string]any{"level": "info", "other": "data"},
			expected: map[string]any{"level": "info", "other": "data"},
		},
		{
			name: "message_as_map_flattens_to_top_level",
			input: map[string]any{
				"message": map[string]any{"nested_key": "nested_val", "foo": "bar", "message": int64(36)},
				"level":   "info",
			},
			expected: map[string]any{
				"nested_key": "nested_val",
				"foo":        "bar",
				"level":      "info",
				"message":    int64(36),
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
			name:     "message_nil_then_compatible_field_promoted",
			input:    map[string]any{"message": nil, "msg": "x"},
			expected: map[string]any{"message": "x"},
		},
		{
			name:     "compatible_field_nil_is_dropped",
			input:    map[string]any{"msg": nil, "level": "info"},
			expected: map[string]any{"level": "info"},
		},
		{
			name:     "compatible_field_nil_falls_through_to_next_field",
			input:    map[string]any{"log": nil, "msg": "request served"},
			expected: map[string]any{"message": "request served"},
		},
		{
			name:     "message_missing_compatible_field_as_map_flattens_after_promotion",
			input:    map[string]any{"msg": map[string]any{"nested_key": "nested_val", "foo": "bar"}, "level": "info"},
			expected: map[string]any{"nested_key": "nested_val", "foo": "bar", "level": "info"},
		},
		{
			name:     "flattened_keys_overwrite_top_level_keys",
			input:    map[string]any{"message": map[string]any{"level": "debug"}, "level": "info"},
			expected: map[string]any{"level": "debug"},
		},
		{
			name:     "message_as_slice_skipped",
			input:    map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
			expected: map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := pcommon.NewMap()
			require.NoError(t, m.FromRaw(tc.input))
			newTestProcessor(t, false).normalizeMessage(m)
			require.Equal(t, tc.expected, m.AsRaw())
		})
	}
}

func TestProcessLogsBody(t *testing.T) {
	cases := []struct {
		name     string
		body     any
		expected map[string]any
	}{
		{
			name:     "json_string_promotes_msg",
			body:     `{"msg": "test message", "level": "info"}`,
			expected: map[string]any{"message": "test message", "level": "info"},
		},
		{
			name:     "text_wrapped_in_message",
			body:     "Hello World",
			expected: map[string]any{"message": "Hello World"},
		},
		{
			name:     "quoted_json_string_is_unquoted_then_parsed",
			body:     `"{\"msg\":\"hi\"}"`,
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "json_with_trailing_whitespace_parsed",
			body:     "{\"msg\":\"hi\"} \t\r\n",
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "json_with_leading_whitespace_parsed",
			body:     " \t\r\n{\"msg\":\"hi\"}",
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "invalid_json_with_surrounding_whitespace_kept_verbatim",
			body:     " {\"a\":1,,}\n",
			expected: map[string]any{"message": " {\"a\":1,,}\n"},
		},
		{
			name:     "quoted_json_with_trailing_whitespace_parsed",
			body:     "\"{\\\"msg\\\":\\\"hi\\n\\\"}\\n\"\n",
			expected: map[string]any{"message": "hi\n"},
		},
		{
			name:     "invalid_json_with_trailing_whitespace_kept_verbatim",
			body:     "{\"a\":1,,}\n",
			expected: map[string]any{"message": "{\"a\":1,,}\n"},
		},
		{
			name:     "text_with_trailing_whitespace_kept_verbatim",
			body:     "Hello World \n",
			expected: map[string]any{"message": "Hello World \n"},
		},
		{
			name:     "invalid_json_object_kept_as_text",
			body:     `{"a":1,,}`,
			expected: map[string]any{"message": `{"a":1,,}`},
		},
		{
			name:     "concatenated_objects_kept_as_text",
			body:     `{"a":1}{"b":2}`,
			expected: map[string]any{"message": `{"a":1}{"b":2}`},
		},
		{
			name:     "json_array_string_kept_as_text",
			body:     `[1,2]`,
			expected: map[string]any{"message": "[1,2]"},
		},
		{
			name:     "empty_object_string_stays_empty",
			body:     `{}`,
			expected: map[string]any{},
		},
		{
			name:     "integers_kept_exact",
			body:     `{"id": 9007199254740993, "ratio": 1.5, "ok": true, "none": null}`,
			expected: map[string]any{"id": int64(9007199254740993), "ratio": 1.5, "ok": true, "none": nil},
		},
		{
			name:     "map_body_promotes_msg",
			body:     map[string]any{"msg": "x", "level": "info"},
			expected: map[string]any{"message": "x", "level": "info"},
		},
		{
			name:     "int_body_wrapped",
			body:     int64(42),
			expected: map[string]any{"message": int64(42)},
		},
		{
			name:     "double_body_wrapped",
			body:     1.25,
			expected: map[string]any{"message": 1.25},
		},
		{
			name:     "bool_body_wrapped",
			body:     true,
			expected: map[string]any{"message": true},
		},
		{
			name:     "bytes_body_wrapped",
			body:     []byte("raw"),
			expected: map[string]any{"message": []byte("raw")},
		},
		{
			name:     "slice_body_wrapped",
			body:     []any{"a", int64(1)},
			expected: map[string]any{"message": []any{"a", int64(1)}},
		},
	}

	p := newTestProcessor(t, false)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lr := processSingle(t, p, tc.body)
			require.Equal(t, pcommon.ValueTypeMap, lr.Body().Type())
			require.Equal(t, tc.expected, lr.Body().AsRaw())
			requireNoStash(t, lr)
		})
	}
}

func TestEmptyBodyIsLeftUntouched(t *testing.T) {
	for _, dual := range []bool{false, true} {
		lr := processSingle(t, newTestProcessor(t, dual), nil)
		require.Equal(t, pcommon.ValueTypeEmpty, lr.Body().Type())
		requireNoStash(t, lr)
	}
}

func TestStashOriginalBodyWhenDualIngestion(t *testing.T) {
	cases := []struct {
		name          string
		body          any
		expectedBody  map[string]any
		expectedStash any
	}{
		{
			name:          "text_stashed_as_is",
			body:          "Hello World",
			expectedBody:  map[string]any{"message": "Hello World"},
			expectedStash: "Hello World",
		},
		{
			name:          "json_string_stashed_byte_exact",
			body:          `{"msg": "hi",   "level": "info"}`,
			expectedBody:  map[string]any{"message": "hi", "level": "info"},
			expectedStash: `{"msg": "hi",   "level": "info"}`,
		},
		{
			name:          "quoted_json_string_stashed_with_quotes",
			body:          `"{\"msg\":\"hi\"}"`,
			expectedBody:  map[string]any{"message": "hi"},
			expectedStash: `"{\"msg\":\"hi\"}"`,
		},
		{
			name:          "invalid_json_stashed_as_is",
			body:          `{"a":1,,}`,
			expectedBody:  map[string]any{"message": `{"a":1,,}`},
			expectedStash: `{"a":1,,}`,
		},
		{
			name:          "int_stashed_typed",
			body:          int64(42),
			expectedBody:  map[string]any{"message": int64(42)},
			expectedStash: int64(42),
		},
		{
			name:          "bytes_stashed_typed",
			body:          []byte("raw"),
			expectedBody:  map[string]any{"message": []byte("raw")},
			expectedStash: []byte("raw"),
		},
		{
			name:          "slice_stashed_typed",
			body:          []any{"a", int64(1)},
			expectedBody:  map[string]any{"message": []any{"a", int64(1)}},
			expectedStash: []any{"a", int64(1)},
		},
		{
			name:          "unmutated_map_stashed_as_legacy_json",
			body:          map[string]any{"message": "x", "level": "info"},
			expectedBody:  map[string]any{"message": "x", "level": "info"},
			expectedStash: `{"level":"info","message":"x"}`,
		},
		{
			name:          "map_with_msg_promotion_stashed_as_legacy_json",
			body:          map[string]any{"msg": "x", "level": "info"},
			expectedBody:  map[string]any{"message": "x", "level": "info"},
			expectedStash: `{"level":"info","msg":"x"}`,
		},
		{
			name:          "map_with_nil_message_stashed_as_legacy_json",
			body:          map[string]any{"message": nil, "level": "info"},
			expectedBody:  map[string]any{"level": "info"},
			expectedStash: `{"level":"info","message":null}`,
		},
		{
			name:          "map_with_message_map_hoist_stashed_as_legacy_json",
			body:          map[string]any{"message": map[string]any{"a": "b"}, "level": "info"},
			expectedBody:  map[string]any{"a": "b", "level": "info"},
			expectedStash: `{"level":"info","message":{"a":"b"}}`,
		},
	}

	p := newTestProcessor(t, true)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lr := processSingle(t, p, tc.body)
			require.Equal(t, tc.expectedBody, lr.Body().AsRaw())
			stash, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
			require.True(t, exists)
			require.Equal(t, tc.expectedStash, stash.AsRaw())
		})
	}
}

func TestMapStashMatchesLegacyBodyStringification(t *testing.T) {
	body := map[string]any{"z": int64(1), "a": map[string]any{"html": "<b>&</b>"}, "msg": "x"}
	legacy := pcommon.NewValueEmpty()
	require.NoError(t, legacy.FromRaw(body))

	lr := processSingle(t, newTestProcessor(t, true), body)
	stash, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
	require.True(t, exists)
	require.Equal(t, legacy.AsString(), stash.Str())
}

func TestStashKeepsExistingAttributes(t *testing.T) {
	ld := newLogsWithBodies(t, "Hello World")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	lr.Attributes().PutStr("k", "v")

	_, err := newTestProcessor(t, true).ProcessLogs(context.Background(), ld)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"k": "v", constants.OriginalBodyAttributeKey: "Hello World"}, lr.Attributes().AsRaw())
}

func TestNoStashWhenDualIngestionDisabled(t *testing.T) {
	p := newTestProcessor(t, false)
	for _, body := range []any{"Hello World", map[string]any{"msg": "x"}, int64(7)} {
		requireNoStash(t, processSingle(t, p, body))
	}
}

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(ctx)) })

	p, err := newNormalizeProcessor(tel.NewTelemetrySettings(), testConfig(false))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.telemetry.shutdown(ctx)) })

	_, err = p.ProcessLogs(ctx, newLogsWithBodies(t,
		"a",
		nil,
		map[string]any{"msg": "x"},
		`{"log":"y"}`,
		`{"message":{"k":"v"}}`,
		`{"message":{"message":7}}`,
		int64(5),
	))
	require.NoError(t, err)

	dp := func(value int64, attrs ...attribute.KeyValue) metricdata.DataPoint[int64] {
		return metricdata.DataPoint[int64]{Value: value, Attributes: attribute.NewSet(attrs...)}
	}
	metadatatest.AssertEqualSignozlogsnormalizeRecords(t, tel, []metricdata.DataPoint[int64]{
		dp(3, attribute.String("body", "json")),
		dp(1, attribute.String("body", "text")),
		dp(1, attribute.String("body", "map")),
		dp(1, attribute.String("body", "other")),
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizeMessagePromotions(t, tel, []metricdata.DataPoint[int64]{
		dp(1, attribute.String("field", "log")),
		dp(1, attribute.String("field", "msg")),
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizeMessageFlattenings(t, tel, []metricdata.DataPoint[int64]{dp(2)}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizeMessageNestedPromotions(t, tel, []metricdata.DataPoint[int64]{dp(1)}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizeMessageStringifications(t, tel, []metricdata.DataPoint[int64]{dp(2)}, metricdatatest.IgnoreTimestamp())
}

func TestShutdownStopsObserving(t *testing.T) {
	ctx := context.Background()
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(ctx)) })

	p, err := newNormalizeProcessor(tel.NewTelemetrySettings(), testConfig(false))
	require.NoError(t, err)
	_, err = tel.GetMetric("otelcol.signozlogsnormalize.records")
	require.NoError(t, err)

	require.NoError(t, p.telemetry.shutdown(ctx))
	_, err = tel.GetMetric("otelcol.signozlogsnormalize.records")
	assert.Error(t, err)
}

func TestFactoryCreatesLogsProcessor(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.False(t, cfg.(*Config).JSONBodyDualIngestion)

	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(ctx, processortest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)
	require.True(t, proc.Capabilities().MutatesData)

	require.NoError(t, proc.Start(ctx, componenttest.NewNopHost()))
	require.NoError(t, proc.ConsumeLogs(ctx, newLogsWithBodies(t, `{"log":"line"}`)))
	require.NoError(t, proc.Shutdown(ctx))

	require.Equal(t, 1, sink.LogRecordCount())
	body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	require.Equal(t, map[string]any{"message": "line"}, body.AsRaw())
}

func TestFactoryRejectsWrongConfigType(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(factory.Type()), struct{}{}, consumertest.NewNop())
	require.Error(t, err)
}

func BenchmarkProcessLogs(b *testing.B) {
	jsonLine := `{"level":"info","msg":"request served","status":200,"path":"/api/v1/items","duration_ms":12.5,"user":{"id":42,"name":"x"}}`
	textLine := `2026-10-05T12:00:00Z INFO request served path=/api/v1/items status=200 duration=12.5ms`
	mapBody := map[string]any{
		"level": "info", "msg": "request served", "status": int64(200), "path": "/api/v1/items", "duration_ms": 12.5,
		"user": map[string]any{"id": int64(42), "name": "x"},
	}
	cases := []struct {
		name string
		body any
	}{
		{name: "json_string", body: jsonLine},
		{name: "text", body: textLine},
		{name: "map", body: mapBody},
	}
	for _, bc := range cases {
		for _, dual := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/dual=%t", bc.name, dual), func(b *testing.B) {
				p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), testConfig(dual))
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					ld := plog.NewLogs()
					lrs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
					for range 1000 {
						if err := lrs.AppendEmpty().Body().FromRaw(bc.body); err != nil {
							b.Fatal(err)
						}
					}
					b.StartTimer()
					if _, err := p.ProcessLogs(context.Background(), ld); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func TestCustomMessageFields(t *testing.T) {
	cfg := testConfig(false)
	cfg.MessageFields = []string{"text"}
	p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), cfg)
	require.NoError(t, err)

	m := pcommon.NewMap()
	require.NoError(t, m.FromRaw(map[string]any{"text": "a", "msg": "b"}))
	p.normalizeMessage(m)
	require.Equal(t, map[string]any{"message": "a", "msg": "b"}, m.AsRaw())
}
