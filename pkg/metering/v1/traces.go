package v1

import (
	"github.com/goccy/go-json"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	"github.com/SigNoz/signoz-otel-collector/pkg/schema/common"
	schema "github.com/SigNoz/signoz-otel-collector/pkg/schema/traces"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traces struct {
	Logger   *zap.Logger
	Sizer    metering.Sizer
	KeySizes map[string]int
}

func NewTraces(logger *zap.Logger) metering.Traces {
	return &traces{
		Logger: logger,
		Sizer:  metering.NewJSONSizer(logger, metering.WithExcludePattern(metering.ExcludeSigNozWorkspaceResourceAttrs)),
		KeySizes: map[string]int{
			"resources_string":  len("\"resources_string\""),
			"startTimeUnixNano": len("\"startTimeUnixNano\""),
			"traceId":           len("\"traceId\""),
			"spanId":            len("\"spanId\""),
			"traceState":        len("\"traceState\""),
			"parentSpanId":      len("\"parentSpanId\""),
			"flags":             len("\"flags\""),
			"name":              len("\"name\""),
			"kind":              len("\"kind\""),
			"spanKind":          len("\"spanKind\""),
			"attributes_string": len("\"attributes_string\""),
			"attributes_bool":   len("\"attributes_bool\""),
			"attributes_number": len("\"attributes_number\""),
			"serviceName":       len("\"serviceName\""),
			"event":             len("\"event\""),
			"references":        len("\"references\""),
		},
	}
}

func (meter *traces) Size(td ptrace.Traces) int {
	total := 0

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		total += meter.SizePerResource(resourceSpan)
	}

	return total
}

func (meter *traces) SizePerResource(rtd ptrace.ResourceSpans) int {
	total := 0
	resourceAttributesSize := meter.Sizer.SizeOfFlatPcommonMapInMapStringString(rtd.Resource().Attributes())

	for j := 0; j < rtd.ScopeSpans().Len(); j++ {
		scopeSpans := rtd.ScopeSpans().At(j)

		for k := 0; k < scopeSpans.Spans().Len(); k++ {
			span := scopeSpans.Spans().At(k)

			// Size of references
			sizeOfOtelSpanRefs := 0
			otelSpanRefs, err := schema.NewOtelSpanRefs(span.Links(), span.ParentSpanID(), span.TraceID())
			if err != nil {
				meter.Logger.Error("cannot create span refs", zap.Error(err))
			} else {
				sizeOfOtelSpanRefs = meter.Sizer.SizeOfOtelSpanRefs(otelSpanRefs)
			}

			// Size of service name
			sizeOfServiceName := 0
			serviceName := common.ServiceName(rtd.Resource())
			serviceNameBytes, err := json.Marshal(serviceName)
			if err != nil {
				meter.Logger.Error("cannot marshal service name", zap.Error(err), zap.String("val", serviceName))
			} else {
				sizeOfServiceName = len(serviceNameBytes)
			}

			// Size of events
			events, _ := schema.NewEventsAndErrorEvents(span.Events(), serviceName, false)
			sizeOfEvents := meter.Sizer.SizeOfEvents(events)

			// Size of attributes
			sizeOfNumberAttributes, sizeOfStringAttributes, sizeOfBoolAttributes := meter.Sizer.SizeOfFlatPcommonMapInNumberStringBool(span.Attributes())

			// Size of flags
			sizeOfFlags := 0
			if span.Flags() != 0 {
				sizeOfFlags = meter.Sizer.SizeOfInt(int(span.Flags()))
			}

			// Let's start making the json object
			// 2({})
			total += 2 +
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["resources_string"], resourceAttributesSize, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["startTimeUnixNano"], meter.Sizer.SizeOfInt(int(span.StartTimestamp())), 2) + //:,
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["spanId"], meter.Sizer.SizeOfSpanID(span.SpanID()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["traceId"], meter.Sizer.SizeOfTraceID(span.TraceID()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["traceState"], len(span.TraceState().AsRaw()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["parentSpanId"], meter.Sizer.SizeOfSpanID(span.ParentSpanID()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["flags"], sizeOfFlags, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["name"], len(span.Name()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["kind"], meter.Sizer.SizeOfInt(int(span.Kind())), 2) + //:,
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["spanKind"], len(span.Kind().String()), 4) + //:"",
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["attributes_string"], sizeOfStringAttributes, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["attributes_bool"], sizeOfBoolAttributes, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["attributes_number"], sizeOfNumberAttributes, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExists(meter.KeySizes["serviceName"], sizeOfServiceName, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["event"], sizeOfEvents, 2) + //:,
				meter.Sizer.TotalSizeIfKeyExistsAndValueIsMapOrSlice(meter.KeySizes["references"], sizeOfOtelSpanRefs, 4) - //:"",
				1 //,
		}

	}

	return total
}

func (*traces) Count(td ptrace.Traces) int {
	return td.SpanCount()
}

func (*traces) CountPerResource(rtd ptrace.ResourceSpans) int {
	spanCount := 0

	ilss := rtd.ScopeSpans()
	for j := 0; j < ilss.Len(); j++ {
		spanCount += ilss.At(j).Spans().Len()
	}

	return spanCount
}
