package traces

import (
	"github.com/SigNoz/signoz-otel-collector/utils"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zapcore"
)

type OtelSpanRef struct {
	TraceId string `json:"traceId,omitempty"`
	SpanId  string `json:"spanId,omitempty"`
	RefType string `json:"refType,omitempty"`
}

func NewOtelSpanRefs(links ptrace.SpanLinkSlice, parentSpanID pcommon.SpanID, traceID pcommon.TraceID) ([]OtelSpanRef, error) {
	parentSpanIDSet := len([8]byte(parentSpanID)) != 0
	if !parentSpanIDSet && links.Len() == 0 {
		return nil, nil
	}

	refsCount := links.Len()
	if parentSpanIDSet {
		refsCount++
	}

	refs := make([]OtelSpanRef, 0, refsCount)

	// Put parent span ID at the first place because usually backends look for it
	// as the first CHILD_OF item in the model.SpanRef slice.
	if parentSpanIDSet {
		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(traceID),
			SpanId:  utils.SpanIDToHexOrEmptyString(parentSpanID),
			RefType: "CHILD_OF",
		})
	}

	for i := 0; i < links.Len(); i++ {
		link := links.At(i)

		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(link.TraceID()),
			SpanId:  utils.SpanIDToHexOrEmptyString(link.SpanID()),

			// Since Jaeger RefType is not captured in internal data,
			// use SpanRefType_FOLLOWS_FROM by default.
			// SpanRefType_CHILD_OF supposed to be set only from parentSpanID.
			RefType: "FOLLOWS_FROM",
		})
	}

	return refs, nil
}

func (r *OtelSpanRef) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("traceId", r.TraceId)
	enc.AddString("spanId", r.SpanId)
	enc.AddString("refType", r.RefType)
	return nil
}
