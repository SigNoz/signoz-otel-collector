package fieldvalues

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/SigNoz/signoz-otel-collector/internal/common/spanfields"
)

func (b *batch) addTraces(td ptrace.Traces) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		var res *resourceRef
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			scopePairs := spanScopePairs(ss)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				seen := b.seenMillis(span.StartTimestamp(), span.EndTimestamp())
				if res == nil {
					r := b.resource(rs.Resource().Attributes(), seen)
					res = &r
				}
				pairs := make([]pair, 0, len(scopePairs)+span.Attributes().Len()+5)
				pairs = append(pairs, scopePairs...)
				pairs = append(pairs,
					stringPair(contextSpan, "name", span.Name()),
					stringPair(contextSpan, "kind_string", span.Kind().String()),
					numberPair(contextSpan, "kind", float64(span.Kind())),
					stringPair(contextSpan, "status_code_string", span.Status().Code().String()),
					numberPair(contextSpan, "status_code", float64(span.Status().Code())),
				)
				pairs = appendCalculatedPairs(pairs, span)
				pairs = appendAttrPairs(pairs, contextAttribute, span.Attributes())

				var eventPairs []pair
				events := span.Events()
				for e := 0; e < events.Len(); e++ {
					ev := events.At(e)
					eventPairs = append(eventPairs, stringPair(contextEvent, "name", ev.Name()))
					eventPairs = appendAttrPairs(eventPairs, contextEvent, ev.Attributes())
				}
				b.record(*res, pairs, eventPairs, seen)
			}
		}
	}
}

func spanScopePairs(ss ptrace.ScopeSpans) []pair {
	var pairs []pair
	if name := ss.Scope().Name(); name != "" {
		pairs = append(pairs, stringPair(contextScope, "scope.name", name))
	}
	if version := ss.Scope().Version(); version != "" {
		pairs = append(pairs, stringPair(contextScope, "scope.version", version))
	}
	return appendAttrPairs(pairs, contextScope, ss.Scope().Attributes())
}

// appendCalculatedPairs adds the span fields that the traces exporter derives
// from attributes and flags, with the same names and values.
func appendCalculatedPairs(pairs []pair, span ptrace.Span) []pair {
	c := spanfields.Calculate(span.Attributes(), int8(span.Kind()))
	for _, f := range []struct{ name, value string }{
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
			pairs = append(pairs, stringPair(contextSpan, f.name, f.value))
		}
	}
	hasError := span.Status().Code() == ptrace.StatusCodeError
	return append(pairs, pair{ctx: contextSpan, name: "has_error", typ: typeBool, str: strconv.FormatBool(hasError)})
}
