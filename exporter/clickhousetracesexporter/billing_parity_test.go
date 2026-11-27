package clickhousetracesexporter

import (
	"math"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	v1 "github.com/SigNoz/signoz-otel-collector/pkg/metering/v1"
)

// The billed span size here and the signozmeterconnector size from pkg/metering must agree.
func TestBilledSpanSizeMatchesMeter(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(ptrace.ResourceSpans, ptrace.Span)
	}{
		{name: "Baseline", mutate: func(ptrace.ResourceSpans, ptrace.Span) {}},
		{name: "KindUnspecified", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetKind(ptrace.SpanKindUnspecified) }},
		{name: "StartTimeZero", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetStartTimestamp(0) }},
		{name: "NoParent", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetParentSpanID(pcommon.SpanID{}) }},
		{name: "Flags", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetFlags(0x301) }},
		{name: "TraceStateEscaped", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.TraceState().FromRaw(`vendor=a<b>&"c"`) }},
		{name: "NameQuote", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetName(`say "hi"`) }},
		{name: "NameHTML", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetName("<module>&") }},
		{name: "NameControl", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetName("a\tb\nc\\d") }},
		{name: "NameUTF8", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetName("héllo 世界") }},
		{name: "NameInvalidUTF8", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.SetName("a\xffb") }},
		{name: "AttrNaNInf", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			sp.Attributes().PutDouble("nan", math.NaN())
			sp.Attributes().PutDouble("inf", math.Inf(1))
		}},
		{name: "AttrMapWithNaN", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			require.NoError(t, sp.Attributes().PutEmptyMap("m").FromRaw(map[string]any{
				"s": "v", "n": 1, "b": true, "arr": []any{1, "x"}, "nil": nil, "nan": math.NaN(), "deep": map[string]any{"k": "v"},
			}))
		}},
		{name: "AttrSliceBytesEmpty", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			s := sp.Attributes().PutEmptySlice("arr")
			s.AppendEmpty().SetStr("a")
			s.AppendEmpty().SetInt(1)
			sp.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 2, 3})
			sp.Attributes().PutEmpty("empty")
		}},
		{name: "AttrGenAINumber", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.Attributes().PutDouble("signoz.gen_ai.cost", 1.25) }},
		{name: "AttrGenAIMapNumber", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			require.NoError(t, sp.Attributes().PutEmptyMap("signoz.gen_ai").FromRaw(map[string]any{"cost": 1.25, "model": "m"}))
		}},
		{name: "AttrGenAIString", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) { sp.Attributes().PutStr("signoz.gen_ai.model", "gpt") }},
		{name: "AttrWorkspace", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			sp.Attributes().PutStr("signoz.workspace.s", "y")
			sp.Attributes().PutInt("signoz.workspace.n", 1)
			sp.Attributes().PutBool("signoz.workspace.b", true)
		}},
		{name: "AttrMapWithWorkspaceKey", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			require.NoError(t, sp.Attributes().PutEmptyMap("signoz").FromRaw(map[string]any{"workspace": map[string]any{"id": "x"}, "other": "y"}))
		}},
		{name: "ResourceWorkspace", mutate: func(rs ptrace.ResourceSpans, _ ptrace.Span) {
			rs.Resource().Attributes().PutStr("signoz.workspace.key.id", "abc")
		}},
		{name: "ResourceGenAI", mutate: func(rs ptrace.ResourceSpans, _ ptrace.Span) {
			rs.Resource().Attributes().PutStr("signoz.gen_ai.x", "abc")
		}},
		{name: "ResourceMapAndNaN", mutate: func(rs ptrace.ResourceSpans, _ ptrace.Span) {
			require.NoError(t, rs.Resource().Attributes().PutEmptyMap("rm").FromRaw(map[string]any{"a": 1, "b": map[string]any{"c": "d"}}))
			rs.Resource().Attributes().PutDouble("rn", math.NaN())
		}},
		{name: "NoServiceName", mutate: func(rs ptrace.ResourceSpans, _ ptrace.Span) { rs.Resource().Attributes().Remove("service.name") }},
		{name: "Events", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			e := sp.Events().AppendEmpty()
			e.SetName("ev")
			e.SetTimestamp(1795694535656865001)
			e.Attributes().PutStr("k", "v")
			e.Attributes().PutDouble("nan", math.NaN())
		}},
		{name: "Exception", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			sp.Status().SetCode(ptrace.StatusCodeError)
			e := sp.Events().AppendEmpty()
			e.SetName("exception")
			e.Attributes().PutStr("exception.type", "Err")
			e.Attributes().PutStr("exception.message", `bad "thing" <x>`)
			e.Attributes().PutStr("exception.stacktrace", "at a\n\tat b")
		}},
		{name: "Link", mutate: func(_ ptrace.ResourceSpans, sp ptrace.Span) {
			l := sp.Links().AppendEmpty()
			l.SetTraceID(pcommon.TraceID{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
			l.SetSpanID(pcommon.SpanID{9, 9, 9, 9, 9, 9, 9, 9})
			l.Attributes().PutStr("k", "v")
		}},
	}

	meter := v1.NewTraces(zap.NewNop())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", "svc")
			rs.Resource().Attributes().PutStr("host.name", "h1")
			sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			sp.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
			sp.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
			sp.SetParentSpanID(pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1})
			sp.SetName("GET /api")
			sp.SetKind(ptrace.SpanKindServer)
			sp.SetStartTimestamp(1795694535656865000)
			sp.SetEndTimestamp(1795694535756865000)
			sp.Attributes().PutStr("http.method", "GET")
			sp.Attributes().PutInt("http.status_code", 200)
			sp.Attributes().PutBool("ok", true)
			sp.Attributes().PutDouble("ratio", 0.5)
			tc.mutate(rs, sp)

			span, err := newStructuredSpanV3(0, "fp", sp, ServiceNameForResource(rs.Resource()), rs.Resource(), rs.ScopeSpans().At(0).Scope(), storageConfig{}, map[string]struct{}{})
			require.NoError(t, err)
			billed, err := json.Marshal(span)
			require.NoError(t, err)

			assert.Equal(t, len(billed), meter.Size(td))
		})
	}
}
