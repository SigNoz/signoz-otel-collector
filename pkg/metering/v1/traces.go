package v1

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	"github.com/SigNoz/signoz-otel-collector/pkg/schema/common"
	schema "github.com/SigNoz/signoz-otel-collector/pkg/schema/traces"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traces struct {
	Logger *zap.Logger
	Sizer  metering.Sizer

	KeySizes map[string]int
}

func NewTraces(logger *zap.Logger) metering.Traces {
	return &traces{
		Logger: logger,
		Sizer:  metering.NewJSONSizer(logger),
		KeySizes: map[string]int{
			"resources_string":  len("resources_string"),
			"startTimeUnixNano": len("startTimeUnixNano"),
			"traceId":           len("traceId"),
			"spanId":            len("spanId"),
			"traceState":        len("traceState"),
			"flags":             len("flags"),
			"name":              len("name"),
			"kind":              len("kind"),
			"spanKind":          len("spanKind"),
		},
	}
}

func (meter *traces) Size(td ptrace.Traces) int {
	total := 0

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resourceAttributesSize := meter.Sizer.SizeOfFlatPcommonMapInMapStringString(resourceSpan.Resource().Attributes())

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)

				sizeOfOtelSpanRefs := 0
				otelSpanRefs, err := schema.NewOtelSpanRefs(span.Links(), span.ParentSpanID(), span.TraceID())
				if err != nil {
					meter.Logger.Error("cannot create span refs", zap.Error(err))
				} else {
					sizeOfOtelSpanRefs = meter.Sizer.SizeOfOtelSpanRefs(otelSpanRefs)
				}

				serviceName := common.ServiceName(resourceSpan.Resource())
				sizeOfServiceName := len(serviceName)

				events, _ := schema.NewEventsAndErrorEvents(span.Events(), serviceName, false)
				sizeOfEvents := meter.Sizer.SizeOfStringSlice(events)

				// Let's start making the json object
				// 2({}) + 5("":"")
				total += 2 +
					(meter.KeySizes["resources_string"] + resourceAttributesSize + 5) +
					(meter.KeySizes["startTimeUnixNano"] + meter.Sizer.SizeOfInt(int(span.StartTimestamp())) + 5) +
					(meter.KeySizes["spanId"] + meter.Sizer.SizeOfSpanID(span.SpanID()) + 5) +
					(meter.KeySizes["traceId"] + meter.Sizer.SizeOfTraceID(span.TraceID()) + 5) +
					(meter.KeySizes["traceState"] + len(span.TraceState().AsRaw()) + 5) +
					(meter.KeySizes["parentSpanId"] + meter.Sizer.SizeOfSpanID(span.ParentSpanID()) + 5) +
					(meter.KeySizes["flags"] + meter.Sizer.SizeOfInt(int(span.Flags())) + 5) +
					(meter.KeySizes["name"] + len(span.Name()) + 5) +
					(meter.KeySizes["kind"] + meter.Sizer.SizeOfInt(int(span.Kind())) + 5) +
					(meter.KeySizes["spanKind"] + len(span.Kind().String()) + 5) +
					(meter.KeySizes["attributes_string"] + meter.KeySizes["attributes_bool"] + meter.KeySizes["attributes_number"] + meter.Sizer.SizeOfFlatPcommonMapInMapStringString(span.Attributes()) + 15) +
					(meter.KeySizes["serviceName"] + sizeOfServiceName + 5) +
					(meter.KeySizes["events"] + sizeOfEvents + 5) +
					(meter.KeySizes["references"] + sizeOfOtelSpanRefs + 5)
			}

		}
	}

	return total
}
func (*traces) Count(td ptrace.Traces) int {
	return td.SpanCount()
}
