package fieldvalues

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/SigNoz/signoz-otel-collector/internal/common/spanfields"
)

func (in *recordsInput) addTraces(td ptrace.Traces) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		res := len(in.resources)
		lo := len(in.pairs)
		in.pairs = appendAttrPairs(in.pairs, contextResource, rs.Resource().Attributes(), "")
		in.resources = append(in.resources, pairRange{lo: lo, hi: len(in.pairs)})
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			scope := pairRange{lo: len(in.pairs)}
			in.pairs = appendSpanScopePairs(in.pairs, ss)
			scope.hi = len(in.pairs)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				lo := len(in.pairs)
				in.pairs = append(in.pairs, in.pairs[scope.lo:scope.hi]...)
				in.pairs = append(in.pairs,
					stringPair(contextSpan, "name", span.Name()),
					stringPair(contextSpan, "kind_string", span.Kind().String()),
					numberPair(contextSpan, "kind", float64(span.Kind())),
					stringPair(contextSpan, "status_code_string", span.Status().Code().String()),
					numberPair(contextSpan, "status_code", float64(span.Status().Code())),
				)
				in.pairs = appendCalculatedPairs(in.pairs, span)
				in.pairs = appendAttrPairs(in.pairs, contextAttribute, span.Attributes(), "")
				mid := len(in.pairs)
				events := span.Events()
				for e := 0; e < events.Len(); e++ {
					ev := events.At(e)
					in.pairs = append(in.pairs, stringPair(contextEvent, "name", ev.Name()))
					in.pairs = appendAttrPairs(in.pairs, contextEvent, ev.Attributes(), "")
				}
				in.records = append(in.records, preparedRecord{
					resource: res,
					ts:       span.StartTimestamp(),
					fallback: span.EndTimestamp(),
					lo:       lo,
					mid:      mid,
					hi:       len(in.pairs),
				})
			}
		}
	}
}

func appendSpanScopePairs(dst []pair, ss ptrace.ScopeSpans) []pair {
	if name := ss.Scope().Name(); name != "" {
		dst = append(dst, stringPair(contextScope, "scope.name", name))
	}
	if version := ss.Scope().Version(); version != "" {
		dst = append(dst, stringPair(contextScope, "scope.version", version))
	}
	return appendAttrPairs(dst, contextScope, ss.Scope().Attributes(), "")
}

// appendCalculatedPairs adds the span fields that the traces exporter derives
// from attributes and flags, with the same names and values.
func appendCalculatedPairs(dst []pair, span ptrace.Span) []pair {
	c := spanfields.Calculate(span.Attributes(), int8(span.Kind()))
	for _, f := range [...]struct{ name, value string }{
		{"http_method", c.HTTPMethod},
		{"http_host", c.HTTPHost},
		{"http_url", c.HTTPURL},
		{"response_status_code", c.ResponseStatusCode},
		{"db_name", c.DBName},
		{"db_operation", c.DBOperation},
		{"external_http_method", c.ExternalHTTPMethod},
		{"external_http_url", c.ExternalHTTPURL},
		{"is_remote", spanfields.IsRemote(span.Flags())},
	} {
		if f.value != "" {
			dst = append(dst, stringPair(contextSpan, f.name, f.value))
		}
	}
	return append(dst, boolPair(contextSpan, "has_error", span.Status().Code() == ptrace.StatusCodeError))
}
