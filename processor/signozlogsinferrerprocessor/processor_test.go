package signozlogsinferrerprocessor

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

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsinferrerprocessor/internal/metadatatest"
)

func newTestProcessor(t *testing.T, cfg *Config) *inferrerProcessor {
	t.Helper()
	p, err := newInferrerProcessor(componenttest.NewNopTelemetrySettings(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.telemetry.shutdown(context.Background())) })
	return p
}

type input struct {
	body     map[string]any
	attrs    map[string]any
	scope    map[string]any
	resource map[string]any
	setup    func(plog.LogRecord)
}

func newLogs(t *testing.T, in input) plog.Logs {
	t.Helper()
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	require.NoError(t, rl.Resource().Attributes().FromRaw(in.resource))
	sl := rl.ScopeLogs().AppendEmpty()
	require.NoError(t, sl.Scope().Attributes().FromRaw(in.scope))
	lr := sl.LogRecords().AppendEmpty()
	if in.body != nil {
		require.NoError(t, lr.Body().SetEmptyMap().FromRaw(in.body))
	}
	require.NoError(t, lr.Attributes().FromRaw(in.attrs))
	if in.setup != nil {
		in.setup(lr)
	}
	return ld
}

func process(t *testing.T, p *inferrerProcessor, ld plog.Logs) plog.Logs {
	t.Helper()
	out, err := p.ProcessLogs(context.Background(), ld)
	require.NoError(t, err)
	return out
}

func onlyRecord(t *testing.T, ld plog.Logs) (plog.ScopeLogs, plog.LogRecord) {
	t.Helper()
	require.Equal(t, 1, ld.LogRecordCount())
	sls := ld.ResourceLogs().At(0).ScopeLogs()
	require.Equal(t, 1, sls.Len())
	return sls.At(0), sls.At(0).LogRecords().At(0)
}

func TestSeverity(t *testing.T) {
	testCases := []struct {
		name       string
		in         input
		wantNumber plog.SeverityNumber
		wantText   string
	}{
		{
			name:       "Level_SetsBothHalves",
			in:         input{body: map[string]any{"level": "warning"}},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "WARN",
		},
		{
			name:       "MixedCaseKeyAndValue_Matched",
			in:         input{body: map[string]any{"Level": "Warning"}},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "WARN",
		},
		{
			name:       "SyslogCritical_Error2InErrorGroup",
			in:         input{body: map[string]any{"level": "critical"}},
			wantNumber: plog.SeverityNumberError2,
			wantText:   "ERROR",
		},
		{
			name:       "NumberAndLevel_BothKeptAsWritten",
			in:         input{body: map[string]any{"severity_number": int64(13), "level": "info"}},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "INFO",
		},
		{
			name:       "NumberAlone_SetsText",
			in:         input{body: map[string]any{"severity_number": int64(18)}},
			wantNumber: plog.SeverityNumberError2,
			wantText:   "ERROR",
		},
		{
			name:       "OtlpEnumSpelling_Parsed",
			in:         input{body: map[string]any{"severity_number": "SEVERITY_NUMBER_WARN"}},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "WARN",
		},
		{
			name:       "NumericString_Parsed",
			in:         input{body: map[string]any{"severitynumber": "9"}},
			wantNumber: plog.SeverityNumberInfo,
			wantText:   "INFO",
		},
		{
			name: "NumberOutOfRange_Ignored",
			in:   input{body: map[string]any{"severity_number": int64(30)}},
		},
		{
			name: "NumberUnderTextField_Ignored",
			in:   input{body: map[string]any{"level": int64(30)}},
		},
		{
			name:       "FractionalNumber_FallsThroughToNextName",
			in:         input{body: map[string]any{"severity_number": 9.9, "severitynumber": 17.0}},
			wantNumber: plog.SeverityNumberError,
			wantText:   "ERROR",
		},
		{
			name:       "ExistingNumberOutOfRange_TextLeftEmpty",
			in:         input{setup: func(lr plog.LogRecord) { lr.SetSeverityNumber(25) }},
			wantNumber: 25,
		},
		{
			name:     "UnknownLevel_KeptTrimmed",
			in:       input{body: map[string]any{"level": "  custom  "}},
			wantText: "custom",
		},
		{
			name:     "UnknownLevel_CappedAt50Bytes",
			in:       input{body: map[string]any{"level": "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijXYZ"}},
			wantText: "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghij",
		},
		{
			name:       "TwoNamesInContainer_HigherRankedWins",
			in:         input{body: map[string]any{"lvl": "debug", "severity": "error"}},
			wantNumber: plog.SeverityNumberError,
			wantText:   "ERROR",
		},
		{
			name:       "UnusableValue_FallsThroughToNextName",
			in:         input{body: map[string]any{"severity": map[string]any{"x": 1}, "level": "info"}},
			wantNumber: plog.SeverityNumberInfo,
			wantText:   "INFO",
		},
		{
			name:       "KeysDifferOnlyInCase_SmallerKeyWins",
			in:         input{body: map[string]any{"LEVEL": "error", "level": "info"}},
			wantNumber: plog.SeverityNumberError,
			wantText:   "ERROR",
		},
		{
			name:       "BodyAndAttributes_BodyWins",
			in:         input{body: map[string]any{"level": "info"}, attrs: map[string]any{"level": "error"}},
			wantNumber: plog.SeverityNumberInfo,
			wantText:   "INFO",
		},
		{
			name:       "AttributesAndScope_AttributesWin",
			in:         input{attrs: map[string]any{"level": "trace"}, scope: map[string]any{"level": "fatal"}},
			wantNumber: plog.SeverityNumberTrace,
			wantText:   "TRACE",
		},
		{
			name:       "ScopeAndResource_ScopeWins",
			in:         input{scope: map[string]any{"level": "notice"}, resource: map[string]any{"level": "error"}},
			wantNumber: plog.SeverityNumberInfo2,
			wantText:   "INFO",
		},
		{
			name:       "ResourceOnly_Used",
			in:         input{resource: map[string]any{"log.level": "debug"}},
			wantNumber: plog.SeverityNumberDebug,
			wantText:   "DEBUG",
		},
		{
			name: "BothHalvesSet_NotOverwritten",
			in: input{
				body: map[string]any{"level": "error", "severity_number": int64(17)},
				setup: func(lr plog.LogRecord) {
					lr.SetSeverityNumber(plog.SeverityNumberInfo)
					lr.SetSeverityText("info")
				},
			},
			wantNumber: plog.SeverityNumberInfo,
			wantText:   "info",
		},
		{
			name: "TextSet_NumberFromLog",
			in: input{
				body:  map[string]any{"severity_number": int64(21)},
				setup: func(lr plog.LogRecord) { lr.SetSeverityText("oops") },
			},
			wantNumber: plog.SeverityNumberFatal,
			wantText:   "oops",
		},
		{
			name: "NumberSet_TextFromLog",
			in: input{
				body:  map[string]any{"level": "error"},
				setup: func(lr plog.LogRecord) { lr.SetSeverityNumber(plog.SeverityNumberWarn) },
			},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "ERROR",
		},
		{
			name:       "TextSetAlone_FillsNumber",
			in:         input{setup: func(lr plog.LogRecord) { lr.SetSeverityText("warn") }},
			wantNumber: plog.SeverityNumberWarn,
			wantText:   "warn",
		},
	}

	p := newTestProcessor(t, &Config{})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, lr := onlyRecord(t, process(t, p, newLogs(t, testCase.in)))
			assert.Equal(t, testCase.wantNumber, lr.SeverityNumber())
			assert.Equal(t, testCase.wantText, lr.SeverityText())
		})
	}
}

func TestTraceContext(t *testing.T) {
	testCases := []struct {
		name      string
		in        input
		wantTrace pcommon.TraceID
		wantSpan  pcommon.SpanID
	}{
		{
			name:      "HexIDs_Parsed",
			in:        input{body: map[string]any{"trace_id": "0102030405060708090a0b0c0d0e0f10", "span_id": "0102030405060708"}},
			wantTrace: pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			wantSpan:  pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name:      "DottedNamesInAttributes_Parsed",
			in:        input{attrs: map[string]any{"trace.id": "ffeeddccbbaa99887766554433221100", "span.id": "a1b2c3d4e5f60718"}},
			wantTrace: pcommon.TraceID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00},
			wantSpan:  pcommon.SpanID{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18},
		},
		{
			name: "RawBytes_Used",
			in: input{attrs: map[string]any{
				"traceid": []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 1},
				"spanid":  []byte{7, 7, 7, 7, 7, 7, 7, 2},
			}},
			wantTrace: pcommon.TraceID{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 1},
			wantSpan:  pcommon.SpanID{7, 7, 7, 7, 7, 7, 7, 2},
		},
		{
			name: "AllZeroIDs_Rejected",
			in:   input{body: map[string]any{"trace_id": "00000000000000000000000000000000", "span_id": "0000000000000000"}},
		},
		{
			name: "WrongLength_Rejected",
			in:   input{body: map[string]any{"trace_id": "0102", "span_id": "0a0b0c0d0e0f00010203040506070809"}},
		},
		{
			name: "NonHex_Rejected",
			in:   input{body: map[string]any{"trace_id": "zz02030405060708090a0b0c0d0e0f10", "span_id": "zz02030405060708"}},
		},
		{
			name:      "InvalidBodyID_FallsThroughToAttributes",
			in:        input{body: map[string]any{"trace_id": "bad"}, attrs: map[string]any{"trace_id": "11111111111111111111111111111111"}},
			wantTrace: pcommon.TraceID{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
		},
		{
			name: "IDsSet_NotOverwritten",
			in: input{
				body: map[string]any{"trace_id": "2222222222222222222222222222222a", "span_id": "333333333333333b"},
				setup: func(lr plog.LogRecord) {
					lr.SetTraceID(pcommon.TraceID{4})
					lr.SetSpanID(pcommon.SpanID{5})
				},
			},
			wantTrace: pcommon.TraceID{4},
			wantSpan:  pcommon.SpanID{5},
		},
	}

	p := newTestProcessor(t, &Config{})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, lr := onlyRecord(t, process(t, p, newLogs(t, testCase.in)))
			assert.Equal(t, testCase.wantTrace, lr.TraceID())
			assert.Equal(t, testCase.wantSpan, lr.SpanID())
		})
	}
}

func TestFieldsAreCopiedNotMoved(t *testing.T) {
	body := map[string]any{
		"level":         "info",
		"trace_id":      "abababababababababababababababab",
		"span_id":       "cdcdcdcdcdcdcdcd",
		"scope.name":    "checkout",
		"scope.version": "3.1",
	}
	attrs := map[string]any{"severity_number": int64(9)}

	_, lr := onlyRecord(t, process(t, newTestProcessor(t, &Config{}), newLogs(t, input{body: body, attrs: attrs})))
	assert.Equal(t, body, lr.Body().Map().AsRaw())
	assert.Equal(t, attrs, lr.Attributes().AsRaw())
}

func TestScope(t *testing.T) {
	testCases := []struct {
		name           string
		in             input
		scopeName      string
		scopeVersion   string
		wantName       string
		wantVersion    string
		wantScopeAttrs map[string]any
	}{
		{
			name:           "NameAndVersionInferred_ScopeAttributesKept",
			in:             input{body: map[string]any{"scope.name": "cart", "scope_version": " 1.2 "}, scope: map[string]any{"team": "core"}},
			wantName:       "cart",
			wantVersion:    "1.2",
			wantScopeAttrs: map[string]any{"team": "core"},
		},
		{
			name:           "NameSet_VersionInferred",
			in:             input{body: map[string]any{"scope.name": "other", "scope.version": "2"}},
			scopeName:      "mine",
			wantName:       "mine",
			wantVersion:    "2",
			wantScopeAttrs: map[string]any{},
		},
		{
			name:           "NameAndVersionSet_NotOverwritten",
			in:             input{body: map[string]any{"scope.name": "other", "scope.version": "9"}},
			scopeName:      "payments",
			scopeVersion:   "1",
			wantName:       "payments",
			wantVersion:    "1",
			wantScopeAttrs: map[string]any{},
		},
		{
			name:           "Logger_NotADefaultName",
			in:             input{body: map[string]any{"logger": "auth"}},
			wantScopeAttrs: map[string]any{},
		},
	}

	p := newTestProcessor(t, &Config{})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ld := newLogs(t, testCase.in)
			scope := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
			scope.SetName(testCase.scopeName)
			scope.SetVersion(testCase.scopeVersion)

			sl, _ := onlyRecord(t, process(t, p, ld))
			assert.Equal(t, testCase.wantName, sl.Scope().Name())
			assert.Equal(t, testCase.wantVersion, sl.Scope().Version())
			assert.Equal(t, testCase.wantScopeAttrs, sl.Scope().Attributes().AsRaw())
		})
	}
}

func TestScopeRegrouping(t *testing.T) {
	t.Run("RecordsGroupedByInferredScope", func(t *testing.T) {
		ld := plog.NewLogs()
		sl := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.30.0")
		for _, name := range []string{"a", "", "b", "a"} {
			m := sl.LogRecords().AppendEmpty().Body().SetEmptyMap()
			m.PutStr("scope_name", name)
			m.PutStr("id", name)
		}

		sls := process(t, newTestProcessor(t, &Config{}), ld).ResourceLogs().At(0).ScopeLogs()
		got := map[string][]string{}
		for i := 0; i < sls.Len(); i++ {
			assert.Equal(t, "https://opentelemetry.io/schemas/1.30.0", sls.At(i).SchemaUrl())
			lrs := sls.At(i).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				id, _ := lrs.At(k).Body().Map().Get("id")
				got[sls.At(i).Scope().Name()] = append(got[sls.At(i).Scope().Name()], id.Str())
			}
		}
		assert.Equal(t, map[string][]string{"": {""}, "a": {"a", "a"}, "b": {"b"}}, got)
	})

	t.Run("EmptiedScope_Removed", func(t *testing.T) {
		ld := plog.NewLogs()
		sls := ld.ResourceLogs().AppendEmpty().ScopeLogs()
		sls.AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptyMap().PutStr("scope_name", "inventory")
		sls.AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("plain line")

		got := process(t, newTestProcessor(t, &Config{}), ld).ResourceLogs().At(0).ScopeLogs()
		require.Equal(t, 2, got.Len())
		assert.Empty(t, got.At(0).Scope().Name())
		assert.Equal(t, "plain line", got.At(0).LogRecords().At(0).Body().Str())
		assert.Equal(t, "inventory", got.At(1).Scope().Name())
	})
}

func TestFieldNames(t *testing.T) {
	testCases := []struct {
		name       string
		cfg        *Config
		body       map[string]any
		wantNumber plog.SeverityNumber
		wantText   string
		wantTrace  pcommon.TraceID
		wantScope  string
	}{
		{
			name:       "Configured_ReplacesDefaults",
			cfg:        &Config{SeverityTextFields: []string{"sev"}, ScopeNameFields: []string{"Logger"}},
			body:       map[string]any{"level": "info", "sev": "error", "logger": "billing"},
			wantNumber: plog.SeverityNumberError,
			wantText:   "ERROR",
			wantScope:  "billing",
		},
		{
			name:       "TextDisabled_NotDerivedFromNumber",
			cfg:        &Config{SeverityTextFields: []string{}},
			body:       map[string]any{"severity_number": int64(9)},
			wantNumber: plog.SeverityNumberInfo,
		},
		{
			name:     "NumberDisabled_NotDerivedFromText",
			cfg:      &Config{SeverityNumberFields: []string{}},
			body:     map[string]any{"level": "warn"},
			wantText: "WARN",
		},
		{
			name:      "Empty_Disabled_Unspecified_Default",
			cfg:       &Config{SeverityTextFields: []string{}},
			body:      map[string]any{"level": "info", "trace_id": "5555555555555555555555555555555f"},
			wantTrace: pcommon.TraceID{0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x5f},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sl, lr := onlyRecord(t, process(t, newTestProcessor(t, testCase.cfg), newLogs(t, input{body: testCase.body})))
			assert.Equal(t, testCase.wantNumber, lr.SeverityNumber())
			assert.Equal(t, testCase.wantText, lr.SeverityText())
			assert.Equal(t, testCase.wantTrace, lr.TraceID())
			assert.Equal(t, testCase.wantScope, sl.Scope().Name())
		})
	}
}

func TestTextBody_SearchesAttributes(t *testing.T) {
	ld := newLogs(t, input{
		attrs: map[string]any{"level": "info"},
		setup: func(lr plog.LogRecord) { lr.Body().SetStr(`{"level":"error"}`) },
	})

	_, lr := onlyRecord(t, process(t, newTestProcessor(t, &Config{}), ld))
	assert.Equal(t, "INFO", lr.SeverityText())
}

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(ctx)) })

	p, err := newInferrerProcessor(tel.NewTelemetrySettings(), &Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.telemetry.shutdown(ctx)) })

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("scope.version", "4.0")
	lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetEmptyMap().PutStr("level", "warn")
	m := lrs.AppendEmpty().Body().SetEmptyMap()
	m.PutStr("Level", "error")
	m.PutInt("severity_number", 17)
	lr := lrs.AppendEmpty()
	lr.Body().SetStr("no fields")
	lr.Attributes().PutStr("span.id", "0f0e0d0c0b0a0908")
	process(t, p, ld)

	dp := func(n int64, target, field, source string) metricdata.DataPoint[int64] {
		return metricdata.DataPoint[int64]{Value: n, Attributes: attribute.NewSet(
			attribute.String("target", target),
			attribute.String("field", field),
			attribute.String("source", source),
		)}
	}
	metadatatest.AssertEqualSignozlogsinferrerLogRecords(t, tel, []metricdata.DataPoint[int64]{{Value: 3}}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualSignozlogsinferrerFieldInferences(t, tel, []metricdata.DataPoint[int64]{
		dp(1, "severity_number", "severity_number", "body"),
		dp(2, "severity_text", "level", "body"),
		dp(1, "span_id", "span.id", "attributes"),
		dp(3, "scope_version", "scope.version", "resource"),
		dp(1, "severity_number", "severity_text", "derived"),
	}, metricdatatest.IgnoreTimestamp())
}

func TestFactory_CreatesLogsProcessor(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()
	proc, err := factory.CreateLogs(ctx, processortest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, proc.Start(ctx, componenttest.NewNopHost()))
	assert.NoError(t, proc.Shutdown(ctx))
}

func BenchmarkProcessLogs(b *testing.B) {
	p, err := newInferrerProcessor(componenttest.NewNopTelemetrySettings(), &Config{})
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, p.telemetry.shutdown(context.Background())) })

	build := func() plog.Logs {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "frontend")
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		for i := 0; i < 1000; i++ {
			lr := lrs.AppendEmpty()
			m := lr.Body().SetEmptyMap()
			m.PutStr("message", "request served")
			m.PutStr("level", "info")
			m.PutStr("trace_id", "4bf92f3577b34da6a3ce929d0e0e4736")
			m.PutStr("span_id", "00f067aa0ba902b7")
			m.PutStr("user", "u-42")
			lr.Attributes().PutStr("host", "node-1")
		}
		return ld
	}

	batches := make([]plog.Logs, b.N)
	for i := range batches {
		batches[i] = build()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.ProcessLogs(context.Background(), batches[i])
	}
}
