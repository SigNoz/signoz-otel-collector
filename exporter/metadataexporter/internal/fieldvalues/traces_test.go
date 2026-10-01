package fieldvalues

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

func spansOf(events ...map[string]any) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("otel-go")
	for _, attrs := range events {
		span := ss.Spans().AppendEmpty()
		span.SetName("GET /cart")
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeError)
		span.SetStartTimestamp(at("10:00"))
		span.Attributes().PutStr("http.method", "GET")
		ev := span.Events().AppendEmpty()
		ev.SetName("exception")
		_ = ev.Attributes().FromRaw(attrs)
	}
	return td
}

func TestTracesIntrinsicsInSetAndEventsOutsideHash(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalTraces)
	require.NoError(t, e.WriteTraces(context.Background(), spansOf(
		map[string]any{"exception.type": "Timeout"},
		map[string]any{"exception.type": "Refused"},
	)))
	assert.Equal(t, []string{
		"{event:exception.type=Refused*, event:exception.type=Timeout*, event:name=exception*, " +
			"has_error=true, http.method=GET, http_method=GET, is_remote=unknown, " +
			"kind=2, kind_string=Server, name=GET /cart, scope.name=otel-go, status_code=2, status_code_string=Error}",
	}, contextSetList(w.take()), "spans that differ only in event fields share one set")
}

// contextSetList is setList with the event context shown, since span events
// and span fields share names such as "name".
func contextSetList(rows []row) []string {
	var renamed []row
	for _, r := range rows {
		if r.p.ctx == contextEvent {
			r.p.name = "event:" + r.p.name
		}
		renamed = append(renamed, r)
	}
	return setList(renamed)
}

func TestTracesCalculatedFields(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalTraces)
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("GET")
	span.SetKind(ptrace.SpanKindClient)
	span.SetFlags(0x100 | 0x200)
	span.SetStartTimestamp(at("10:00"))
	require.NoError(t, span.Attributes().FromRaw(map[string]any{
		"url.full":            "https://payments.example.com/pay",
		"http.request.method": "POST",
		"http.status_code":    502,
		"db.namespace":        "orders",
	}))
	require.NoError(t, e.WriteTraces(context.Background(), td))

	got := map[string]string{}
	for _, r := range w.take() {
		if r.p.ctx == contextSpan {
			got[r.p.name] = describe(r.p)
		}
	}
	assert.Equal(t, map[string]string{
		"name":                 "name=GET",
		"kind_string":          "kind_string=Client",
		"kind":                 "kind=3",
		"status_code_string":   "status_code_string=Unset",
		"status_code":          "status_code=0",
		"http_method":          "http_method=POST",
		"http_host":            "http_host=payments.example.com",
		"http_url":             "http_url=https://payments.example.com/pay",
		"response_status_code": "response_status_code=502",
		"db_name":              "db_name=orders",
		"external_http_method": "external_http_method=POST",
		"external_http_url":    "external_http_url=payments.example.com",
		"is_remote":            "is_remote=yes",
		"has_error":            "has_error=false",
	}, got, "the span fields that the traces exporter derives, with the same names")
}
