package signozlogsnormalizerprocessor

import (
	"context"
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
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsnormalizerprocessor/internal/metadatatest"
)

func testConfig() *Config {
	return createDefaultConfig().(*Config)
}

func newTestProcessor(t *testing.T) *normalizeProcessor {
	t.Helper()
	p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), testConfig())
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

func assertNoStash(t *testing.T, lr plog.LogRecord) {
	t.Helper()
	_, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
	assert.False(t, exists)
}

func TestNormalizeMessage(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "MessageString_Unchanged",
			input:    map[string]any{"message": "hello", "level": "info"},
			expected: map[string]any{"message": "hello", "level": "info"},
		},
		{
			name:     "MessageMissing_MsgPromoted",
			input:    map[string]any{"msg": "test message", "level": "info"},
			expected: map[string]any{"message": "test message", "level": "info"},
		},
		{
			name:     "MessagePresent_LogUntouched",
			input:    map[string]any{"message": "message content", "log": "log content", "other": "data"},
			expected: map[string]any{"message": "message content", "log": "log content", "other": "data"},
		},
		{
			name:     "MessageMissing_LogAndMsg_LogPromoted",
			input:    map[string]any{"log": "from log", "msg": "from msg"},
			expected: map[string]any{"message": "from log", "msg": "from msg"},
		},
		{
			name:     "MessageMissing_NonStringField_Promoted",
			input:    map[string]any{"msg": int64(123), "log": int64(456)},
			expected: map[string]any{"message": int64(456), "msg": int64(123)},
		},
		{
			name:     "MessageMissing_NoMessageFields_Unchanged",
			input:    map[string]any{"level": "info", "other": "data"},
			expected: map[string]any{"level": "info", "other": "data"},
		},
		{
			name: "MessageMap_InnerMessageInt_Flattened",
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
			name: "MessageMap_Flattened_MessageRemoved",
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
			name: "MessageMap_InnerMessageMap_FlattenedOnce",
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
			name:     "MessageNil_Removed",
			input:    map[string]any{"message": nil, "level": "info"},
			expected: map[string]any{"level": "info"},
		},
		{
			name:     "MessageNil_MsgPromoted",
			input:    map[string]any{"message": nil, "msg": "x"},
			expected: map[string]any{"message": "x"},
		},
		{
			name:     "MessageFieldNil_Dropped",
			input:    map[string]any{"msg": nil, "level": "info"},
			expected: map[string]any{"level": "info"},
		},
		{
			name:     "MessageFieldNil_NextFieldPromoted",
			input:    map[string]any{"log": nil, "msg": "request served"},
			expected: map[string]any{"message": "request served"},
		},
		{
			name:     "MessageMissing_MsgMap_PromotedThenFlattened",
			input:    map[string]any{"msg": map[string]any{"nested_key": "nested_val", "foo": "bar"}, "level": "info"},
			expected: map[string]any{"nested_key": "nested_val", "foo": "bar", "level": "info"},
		},
		{
			name:     "MessageMap_KeyCollision_InnerWins",
			input:    map[string]any{"message": map[string]any{"level": "debug"}, "level": "info"},
			expected: map[string]any{"level": "debug"},
		},
		{
			name:     "MessageSlice_Unchanged",
			input:    map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
			expected: map[string]any{"message": []any{"a", "b", "c"}, "level": "info"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			require.NoError(t, m.FromRaw(testCase.input))
			newTestProcessor(t).normalizeMessage(m)
			assert.Equal(t, testCase.expected, m.AsRaw())
		})
	}
}

func TestProcessLogsBody(t *testing.T) {
	testCases := []struct {
		name     string
		body     any
		expected map[string]any
	}{
		{
			name:     "JSONString_MsgPromoted",
			body:     `{"msg": "test message", "level": "info"}`,
			expected: map[string]any{"message": "test message", "level": "info"},
		},
		{
			name:     "Text_WrappedInMessage",
			body:     "Hello World",
			expected: map[string]any{"message": "Hello World"},
		},
		{
			name:     "QuotedJSONString_UnquotedThenParsed",
			body:     `"{\"msg\":\"hi\"}"`,
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "JSONString_TrailingWhitespace_Parsed",
			body:     "{\"msg\":\"hi\"} \t\r\n",
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "JSONString_LeadingWhitespace_Parsed",
			body:     " \t\r\n{\"msg\":\"hi\"}",
			expected: map[string]any{"message": "hi"},
		},
		{
			name:     "InvalidJSON_SurroundingWhitespace_KeptVerbatim",
			body:     " {\"a\":1,,}\n",
			expected: map[string]any{"message": " {\"a\":1,,}\n"},
		},
		{
			name:     "QuotedJSONString_TrailingWhitespace_Parsed",
			body:     "\"{\\\"msg\\\":\\\"hi\\n\\\"}\\n\"\n",
			expected: map[string]any{"message": "hi\n"},
		},
		{
			name:     "InvalidJSON_TrailingWhitespace_KeptVerbatim",
			body:     "{\"a\":1,,}\n",
			expected: map[string]any{"message": "{\"a\":1,,}\n"},
		},
		{
			name:     "Text_TrailingWhitespace_KeptVerbatim",
			body:     "Hello World \n",
			expected: map[string]any{"message": "Hello World \n"},
		},
		{
			name:     "InvalidJSON_KeptAsText",
			body:     `{"a":1,,}`,
			expected: map[string]any{"message": `{"a":1,,}`},
		},
		{
			name:     "ConcatenatedObjects_KeptAsText",
			body:     `{"a":1}{"b":2}`,
			expected: map[string]any{"message": `{"a":1}{"b":2}`},
		},
		{
			name:     "JSONArray_KeptAsText",
			body:     `[1,2]`,
			expected: map[string]any{"message": "[1,2]"},
		},
		{
			name:     "EmptyObject_StaysEmpty",
			body:     `{}`,
			expected: map[string]any{},
		},
		{
			name:     "LargeInteger_KeptExact",
			body:     `{"id": 9007199254740993, "ratio": 1.5, "ok": true, "none": null}`,
			expected: map[string]any{"id": int64(9007199254740993), "ratio": 1.5, "ok": true, "none": nil},
		},
		{
			name:     "MapBody_MsgPromoted",
			body:     map[string]any{"msg": "x", "level": "info"},
			expected: map[string]any{"message": "x", "level": "info"},
		},
		{
			name:     "IntBody_WrappedInMessage",
			body:     int64(42),
			expected: map[string]any{"message": int64(42)},
		},
		{
			name:     "DoubleBody_WrappedInMessage",
			body:     1.25,
			expected: map[string]any{"message": 1.25},
		},
		{
			name:     "BoolBody_WrappedInMessage",
			body:     true,
			expected: map[string]any{"message": true},
		},
		{
			name:     "BytesBody_WrappedInMessage",
			body:     []byte("raw"),
			expected: map[string]any{"message": []byte("raw")},
		},
		{
			name:     "SliceBody_WrappedInMessage",
			body:     []any{"a", int64(1)},
			expected: map[string]any{"message": []any{"a", int64(1)}},
		},
	}

	p := newTestProcessor(t)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lr := processSingle(t, p, testCase.body)
			assert.Equal(t, pcommon.ValueTypeMap, lr.Body().Type())
			assert.Equal(t, testCase.expected, lr.Body().AsRaw())
		})
	}
}

func TestEmptyBodyIsLeftUntouched(t *testing.T) {
	lr := processSingle(t, newTestProcessor(t), nil)
	assert.Equal(t, pcommon.ValueTypeEmpty, lr.Body().Type())
	assertNoStash(t, lr)
}

func TestStashOriginalBody(t *testing.T) {
	testCases := []struct {
		name          string
		body          any
		expectedBody  map[string]any
		expectedStash any
	}{
		{
			name:          "Text_StashedAsIs",
			body:          "Hello World",
			expectedBody:  map[string]any{"message": "Hello World"},
			expectedStash: "Hello World",
		},
		{
			name:          "JSONString_StashedByteExact",
			body:          `{"msg": "hi",   "level": "info"}`,
			expectedBody:  map[string]any{"message": "hi", "level": "info"},
			expectedStash: `{"msg": "hi",   "level": "info"}`,
		},
		{
			name:          "QuotedJSONString_StashedWithQuotes",
			body:          `"{\"msg\":\"hi\"}"`,
			expectedBody:  map[string]any{"message": "hi"},
			expectedStash: `"{\"msg\":\"hi\"}"`,
		},
		{
			name:          "InvalidJSON_StashedAsIs",
			body:          `{"a":1,,}`,
			expectedBody:  map[string]any{"message": `{"a":1,,}`},
			expectedStash: `{"a":1,,}`,
		},
		{
			name:          "IntBody_StashedTyped",
			body:          int64(42),
			expectedBody:  map[string]any{"message": int64(42)},
			expectedStash: int64(42),
		},
		{
			name:          "BytesBody_StashedTyped",
			body:          []byte("raw"),
			expectedBody:  map[string]any{"message": []byte("raw")},
			expectedStash: []byte("raw"),
		},
		{
			name:          "SliceBody_StashedTyped",
			body:          []any{"a", int64(1)},
			expectedBody:  map[string]any{"message": []any{"a", int64(1)}},
			expectedStash: []any{"a", int64(1)},
		},
		{
			name:          "MapBody_Unmutated_StashedAsLegacyJSON",
			body:          map[string]any{"message": "x", "level": "info"},
			expectedBody:  map[string]any{"message": "x", "level": "info"},
			expectedStash: `{"level":"info","message":"x"}`,
		},
		{
			name:          "MapBody_MsgPromoted_StashedAsLegacyJSON",
			body:          map[string]any{"msg": "x", "level": "info"},
			expectedBody:  map[string]any{"message": "x", "level": "info"},
			expectedStash: `{"level":"info","msg":"x"}`,
		},
		{
			name:          "MapBody_MessageNil_StashedAsLegacyJSON",
			body:          map[string]any{"message": nil, "level": "info"},
			expectedBody:  map[string]any{"level": "info"},
			expectedStash: `{"level":"info","message":null}`,
		},
		{
			name:          "MapBody_MessageMapFlattened_StashedAsLegacyJSON",
			body:          map[string]any{"message": map[string]any{"a": "b"}, "level": "info"},
			expectedBody:  map[string]any{"a": "b", "level": "info"},
			expectedStash: `{"level":"info","message":{"a":"b"}}`,
		},
	}

	p := newTestProcessor(t)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lr := processSingle(t, p, testCase.body)
			assert.Equal(t, testCase.expectedBody, lr.Body().AsRaw())
			stash, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
			require.True(t, exists)
			assert.Equal(t, testCase.expectedStash, stash.AsRaw())
		})
	}
}

func TestMapStashMatchesLegacyBodyStringification(t *testing.T) {
	body := map[string]any{"z": int64(1), "a": map[string]any{"html": "<b>&</b>"}, "msg": "x"}
	legacy := pcommon.NewValueEmpty()
	require.NoError(t, legacy.FromRaw(body))

	lr := processSingle(t, newTestProcessor(t), body)
	stash, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
	require.True(t, exists)
	assert.Equal(t, legacy.AsString(), stash.Str())
}

func TestStashKeepsExistingAttributes(t *testing.T) {
	ld := newLogsWithBodies(t, "Hello World")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	lr.Attributes().PutStr("k", "v")

	_, err := newTestProcessor(t).ProcessLogs(context.Background(), ld)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k": "v", constants.OriginalBodyAttributeKey: "Hello World"}, lr.Attributes().AsRaw())
}

func TestIncomingStash(t *testing.T) {
	testCases := []struct {
		name          string
		bodyDisabled  bool
		body          any
		expectedStash any
	}{
		{name: "BodyDisabled_Kept", bodyDisabled: true, body: "untouched line", expectedStash: "stale upstream stash"},
		{name: "EmptyBody_Removed", body: nil},
		{name: "Overwritten", body: "current line", expectedStash: "current line"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ld := newLogsWithBodies(t, testCase.body)
			lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			lr.Attributes().PutStr(constants.OriginalBodyAttributeKey, "stale upstream stash")

			cfg := testConfig()
			cfg.Body.Enabled = !testCase.bodyDisabled
			p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), cfg)
			require.NoError(t, err)
			_, err = p.ProcessLogs(context.Background(), ld)
			require.NoError(t, err)

			stash, exists := lr.Attributes().Get(constants.OriginalBodyAttributeKey)
			if testCase.expectedStash == nil {
				assert.False(t, exists)
				return
			}
			require.True(t, exists)
			assert.Equal(t, testCase.expectedStash, stash.AsRaw())
		})
	}
}

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(ctx)) })

	p, err := newNormalizeProcessor(tel.NewTelemetrySettings(), testConfig())
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
	metadatatest.AssertEqualSignozlogsnormalizerRecords(t, tel, []metricdata.DataPoint[int64]{
		dp(3, attribute.String("body", "json")),
		dp(1, attribute.String("body", "text")),
		dp(1, attribute.String("body", "map")),
		dp(1, attribute.String("body", "other")),
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizerMessagePromotions(t, tel, []metricdata.DataPoint[int64]{
		dp(1, attribute.String("field", "log")),
		dp(1, attribute.String("field", "msg")),
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizerMessageFlattenings(t, tel, []metricdata.DataPoint[int64]{dp(2)}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizerMessageNestedPromotions(t, tel, []metricdata.DataPoint[int64]{dp(1)}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsnormalizerMessageStringifications(t, tel, []metricdata.DataPoint[int64]{dp(2)}, metricdatatest.IgnoreTimestamp())
}

func TestShutdownStopsObserving(t *testing.T) {
	ctx := context.Background()
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(ctx)) })

	p, err := newNormalizeProcessor(tel.NewTelemetrySettings(), testConfig())
	require.NoError(t, err)
	_, err = tel.GetMetric("otelcol.signozlogsnormalizer.records")
	require.NoError(t, err)

	require.NoError(t, p.telemetry.shutdown(ctx))
	_, err = tel.GetMetric("otelcol.signozlogsnormalizer.records")
	assert.Error(t, err)
}

func TestFactoryCreatesLogsProcessor(t *testing.T) {
	testCases := []struct {
		name         string
		bodyDisabled bool
		input        string
		expected     any
	}{
		{name: "DefaultConfig_BodyNormalized", input: `{"log":"line"}`, expected: map[string]any{"message": "line"}},
		{name: "BodyDisabled_BodyUntouched", bodyDisabled: true, input: `{"log":"raw line"}`, expected: `{"log":"raw line"}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			assert.True(t, cfg.Body.Enabled)
			assert.True(t, cfg.Fields.Enabled)
			cfg.Body.Enabled = !testCase.bodyDisabled

			sink := new(consumertest.LogsSink)
			proc, err := factory.CreateLogs(ctx, processortest.NewNopSettings(factory.Type()), cfg, sink)
			require.NoError(t, err)
			assert.True(t, proc.Capabilities().MutatesData)

			require.NoError(t, proc.Start(ctx, componenttest.NewNopHost()))
			require.NoError(t, proc.ConsumeLogs(ctx, newLogsWithBodies(t, testCase.input)))
			require.NoError(t, proc.Shutdown(ctx))

			require.Equal(t, 1, sink.LogRecordCount())
			body := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
			assert.Equal(t, testCase.expected, body.AsRaw())
		})
	}
}

func TestFactoryRejectsWrongConfigType(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(factory.Type()), struct{}{}, consumertest.NewNop())
	assert.Error(t, err)
}

func BenchmarkProcessLogs(b *testing.B) {
	testCases := []struct {
		name string
		body any
	}{
		{name: "JSONString", body: `{"level":"info","msg":"request served","status":200,"path":"/api/v1/items","duration_ms":12.5,"user":{"id":42,"name":"x"}}`},
		{name: "Text", body: `2026-10-05T12:00:00Z INFO request served path=/api/v1/items status=200 duration=12.5ms`},
		{
			name: "MapBody",
			body: map[string]any{
				"level": "info", "msg": "request served", "status": int64(200), "path": "/api/v1/items", "duration_ms": 12.5,
				"user": map[string]any{"id": int64(42), "name": "x"},
			},
		},
	}
	for _, testCase := range testCases {
		b.Run(testCase.name, func(b *testing.B) {
			p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), testConfig())
			require.NoError(b, err)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				ld := plog.NewLogs()
				lrs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
				for range 1000 {
					require.NoError(b, lrs.AppendEmpty().Body().FromRaw(testCase.body))
				}
				b.StartTimer()
				_, err := p.ProcessLogs(context.Background(), ld)
				require.NoError(b, err)
			}
		})
	}
}

func TestCustomMessageFields(t *testing.T) {
	cfg := testConfig()
	cfg.Body.MessageFields = []string{"text"}
	p, err := newNormalizeProcessor(componenttest.NewNopTelemetrySettings(), cfg)
	require.NoError(t, err)

	m := pcommon.NewMap()
	require.NoError(t, m.FromRaw(map[string]any{"text": "a", "msg": "b"}))
	p.normalizeMessage(m)
	assert.Equal(t, map[string]any{"message": "a", "msg": "b"}, m.AsRaw())
}
