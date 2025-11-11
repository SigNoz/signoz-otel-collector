package signozspanmetricsprocessor

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

var lateSpanBuckets = []bucketDefinition{
	{name: "5m-10m", min: 5 * time.Minute, max: 10 * time.Minute},
	{name: "10m-30m", min: 10 * time.Minute, max: 30 * time.Minute},
	{name: "30m-2h", min: 30 * time.Minute, max: 2 * time.Hour},
	{name: "2h-6h", min: 2 * time.Hour, max: 6 * time.Hour},
	{name: "6h-1d", min: 6 * time.Hour, max: 24 * time.Hour},
}

const maxServiceSamplesPerBucket = 3

type bucketDefinition struct {
	name string
	min  time.Duration
	max  time.Duration
}

type spanSummary struct {
	Service      string            `json:"service"`
	Operation    string            `json:"operation"`
	SpanKind     string            `json:"span_kind"`
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	SpanName     string            `json:"span_name"`
	StartTime    string            `json:"start_time"`
	DelaySeconds int               `json:"delay_seconds"`
	ResourceInfo map[string]string `json:"resource_info,omitempty"`
}

type serviceSample struct {
	Count           int      `json:"count"`
	SampleSpanNames []string `json:"sample_span_names,omitempty"`
}

type lateSpanBucketStats struct {
	Count      int
	FirstSpan  spanSummary
	MinDelay   time.Duration
	MinSpan    spanSummary
	MaxDelay   time.Duration
	MaxSpan    spanSummary
	ServiceMap map[string]*serviceSample
}

type lateSpanBucketReport struct {
	Bucket          string                   `json:"bucket"`
	Count           int                      `json:"count"`
	FirstSpan       spanSummary              `json:"first_span"`
	MinDelaySeconds int                      `json:"min_delay_seconds"`
	MinSpan         spanSummary              `json:"min_span"`
	MaxDelaySeconds int                      `json:"max_delay_seconds"`
	MaxSpan         spanSummary              `json:"max_span"`
	ServiceStats    map[string]serviceSample `json:"service_stats"`
}

func bucketForSpanDelay(delay time.Duration) (bucketDefinition, bool) {
	for _, bucket := range lateSpanBuckets {
		if delay >= bucket.min && delay < bucket.max {
			return bucket, true
		}
	}
	return bucketDefinition{}, false
}

func buildSpanSummary(serviceName string, span ptrace.Span, resourceAttr pcommon.Map, delay time.Duration) spanSummary {
	resourceInfo := extractSpanResourceInfo(resourceAttr)
	return spanSummary{
		Service:      serviceName,
		Operation:    span.Name(),
		SpanKind:     span.Kind().String(),
		SpanName:     span.Name(),
		TraceID:      span.TraceID().String(),
		SpanID:       span.SpanID().String(),
		StartTime:    span.StartTimestamp().AsTime().UTC().Format(time.RFC3339Nano),
		DelaySeconds: int(delay.Seconds()),
		ResourceInfo: resourceInfo,
	}
}

func extractSpanResourceInfo(resourceAttr pcommon.Map) map[string]string {
	keysOfInterest := []string{
		string(conventions.AttributeServiceNamespace),
		string(conventions.AttributeDeploymentEnvironment),
		signozID,
	}
	info := make(map[string]string)
	for _, key := range keysOfInterest {
		if value, ok := resourceAttr.Get(key); ok {
			info[key] = value.AsString()
		}
	}
	if len(info) == 0 {
		return nil
	}
	return info
}
