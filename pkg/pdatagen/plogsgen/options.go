package plogsgen

import "go.opentelemetry.io/collector/pdata/pcommon"

type generationOptions struct {
	logRecordCount               int
	resourceAttributeCount       int
	body                         string
	resourceAttributeStringValue string
	timestamp                    pcommon.Timestamp
	observedTimestamp            pcommon.Timestamp
}

type GenerationOption func(*generationOptions)

func WithLogRecordCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.logRecordCount = i
	}
}

func WithResourceAttributeCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeCount = i
	}
}

func WithBody(s string) GenerationOption {
	return func(o *generationOptions) {
		o.body = s
	}
}

func WithResourceAttributeStringValue(s string) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeStringValue = s
	}
}

func WithTimestamp(ts pcommon.Timestamp) GenerationOption {
	return func(o *generationOptions) {
		o.timestamp = ts
	}
}

func WithObservedTimestamp(ts pcommon.Timestamp) GenerationOption {
	return func(o *generationOptions) {
		o.observedTimestamp = ts
	}
}
