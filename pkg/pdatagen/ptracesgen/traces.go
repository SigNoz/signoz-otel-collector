package ptracesgen

import (
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Generate(opts ...GenerationOption) ptrace.Traces {
	generationOpts := generationOptions{
		eventCount:                   0,
		spanCount:                    1,
		resourceAttributeCount:       1,
		resourceAttributeStringValue: "resource",
		spanKind:                     ptrace.SpanKindClient,
		attributes:                   map[string]any{},
	}

	for _, opt := range opts {
		opt(&generationOpts)
	}

	endTime := pcommon.NewTimestampFromTime(time.Now())
	traces := ptrace.NewTraces()
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	for i := 0; i < generationOpts.resourceAttributeCount; i++ {
		suffix := strconv.Itoa(i)
		// Do not change the key name format in resource attributes below.
		resourceSpan.Resource().Attributes().PutStr("resource."+suffix, generationOpts.resourceAttributeStringValue)
	}

	scopeSpans := resourceSpan.ScopeSpans().AppendEmpty()
	scopeSpans.Spans().EnsureCapacity(generationOpts.spanCount)
	for i := 0; i < generationOpts.spanCount; i++ {
		suffix := strconv.Itoa(i)
		span := scopeSpans.Spans().AppendEmpty()
		span.SetName("span." + suffix)
		span.SetKind(generationOpts.spanKind)
		span.SetStartTimestamp(endTime)
		span.SetEndTimestamp(endTime)
		span.SetTraceID(pcommon.TraceID([]byte("5B8EFFF798038103D269B633813FC60C")))
		span.SetSpanID(pcommon.SpanID([]byte("EEE19B7EC3C1B174")))
		span.SetParentSpanID(pcommon.SpanID([]byte("EEE19B7EC3C1B174")))

		for j := 0; j < generationOpts.eventCount; j++ {
			suffix := strconv.Itoa(j)
			spanEvent := span.Events().AppendEmpty()
			spanEvent.SetName("event." + suffix)
			spanEvent.SetTimestamp(endTime)
		}

		for k, v := range generationOpts.attributes {
			switch v := v.(type) {
			case string:
				span.Attributes().PutStr(k, v)
			case float64:
				span.Attributes().PutDouble(k, v)
			case bool:
				span.Attributes().PutBool(k, v)
			case int64:
				span.Attributes().PutInt(k, v)
			}

		}
	}

	return traces
}
