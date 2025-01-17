package ptracesgen

import "go.opentelemetry.io/collector/pdata/ptrace"

type generationOptions struct {
	spanCount                    int
	eventCount                   int
	resourceAttributeCount       int
	resourceAttributeStringValue string
	spanKind                     ptrace.SpanKind
	attributes                   map[string]any
}

type GenerationOption func(*generationOptions)

func WithSpanCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.spanCount = i
	}
}

func WithEventCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.eventCount = i
	}
}

func WithSpanKind(k ptrace.SpanKind) GenerationOption {
	return func(o *generationOptions) {
		o.spanKind = k
	}
}

func WithResourceAttributeCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeCount = i
	}
}

func WithResourceAttributeStringValue(s string) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeStringValue = s
	}
}

func WithAttributes(m map[string]any) GenerationOption {
	return func(o *generationOptions) {
		o.attributes = m
	}
}
