package v1

import (
	"fmt"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/ptracesgen"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestTracesSizeWithNoEvents(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(1),
		ptracesgen.WithResourceAttributeCount(1),
		// 4: SPAN_KIND_PRODUCER
		ptracesgen.WithSpanKind(ptrace.SpanKindProducer),
		ptracesgen.WithResourceAttributeStringValue("test"),
	)

	meter := NewTraces(zap.NewNop())
	size := meter.Size(traces)

	assert.Equal(t, 406, size)
}

func TestTracesSizeWithNoEventAndSigNozResource(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(1),
		ptracesgen.WithResourceAttributeCount(1),
		ptracesgen.WithSpanKind(ptrace.SpanKindProducer),
		ptracesgen.WithResourceAttributeStringValue("test"),
	)
	// adding signoz resource attribute shouldn't affect the calculation
	traces.ResourceSpans().At(0).Resource().Attributes().PutStr("signoz.workspace.internal.test", "signoz-test")

	meter := NewTraces(zap.NewNop())
	size := meter.Size(traces)

	assert.Equal(t, 406, size)
}

func TestTracesSizeWithEvents(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(1),
		ptracesgen.WithResourceAttributeCount(1),
		ptracesgen.WithEventCount(2),
		// 4: SPAN_KIND_PRODUCER
		ptracesgen.WithSpanKind(ptrace.SpanKindProducer),
		ptracesgen.WithResourceAttributeStringValue("test"),
	)

	meter := NewTraces(zap.NewNop())
	size := meter.Size(traces)

	assert.Equal(t, 540, size)
}

func TestTracesSizeWith2SpansAnd2EventsAnd2ResourceAttributes(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(2),
		ptracesgen.WithResourceAttributeCount(2),
		ptracesgen.WithEventCount(2),
		// 4: SPAN_KIND_PRODUCER
		ptracesgen.WithSpanKind(ptrace.SpanKindProducer),
		ptracesgen.WithResourceAttributeStringValue("test"),
	)

	meter := NewTraces(zap.NewNop())

	b, _ := (&ptrace.JSONMarshaler{}).MarshalTraces(traces)
	fmt.Println(string(b))
	size := meter.Size(traces)

	assert.Equal(t, 1120, size)
}

func TestTracesSizeWith2SpansAnd2EventsAnd2ResourceAttributesAndAttributes(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(2),
		ptracesgen.WithResourceAttributeCount(2),
		ptracesgen.WithEventCount(2),
		ptracesgen.WithSpanKind(ptrace.SpanKindClient),
		ptracesgen.WithResourceAttributeStringValue("test"),
		ptracesgen.WithAttributes(map[string]any{
			"float64": 342.5,
			"int64":   int64(342),
			"string":  "attribute",
			"bool":    false,
		}),
	)

	meter := NewTraces(zap.NewNop())

	b, _ := (&ptrace.JSONMarshaler{}).MarshalTraces(traces)
	fmt.Println(string(b))
	size := meter.Size(traces)

	assert.Equal(t, 1368, size)
}

func TestTracesSizeWithBoolAttributes(t *testing.T) {
	traces := ptracesgen.Generate(
		ptracesgen.WithSpanCount(1),
		ptracesgen.WithResourceAttributeCount(1),
		ptracesgen.WithSpanKind(ptrace.SpanKindClient),
		ptracesgen.WithResourceAttributeStringValue("test"),
		ptracesgen.WithAttributes(map[string]any{
			"bool1": false,
			"bool2": true,
		}),
	)

	meter := NewTraces(zap.NewNop())
	b, _ := (&ptrace.JSONMarshaler{}).MarshalTraces(traces)
	fmt.Println(string(b))
	size := meter.Size(traces)

	assert.Equal(t, 451, size)
}
