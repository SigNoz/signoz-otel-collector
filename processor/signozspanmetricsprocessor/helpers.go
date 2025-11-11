package signozspanmetricsprocessor

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"go.uber.org/zap"
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
	DelayMs      int64             `json:"delay_ms"`
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
	Bucket       string                   `json:"bucket"`
	Count        int                      `json:"count"`
	FirstSpan    spanSummary              `json:"first_span"`
	MinDelayMs   int64                    `json:"min_delay_ms"`
	MinSpan      spanSummary              `json:"min_span"`
	MaxDelayMs   int64                    `json:"max_delay_ms"`
	MaxSpan      spanSummary              `json:"max_span"`
	ServiceStats map[string]serviceSample `json:"service_stats"`
}

func (p *processorImp) recordLateSpan(serviceName string, span ptrace.Span, resourceAttr pcommon.Map, delay time.Duration) {
	bucketDef, ok := bucketForSpanDelay(delay)
	if !ok {
		return
	}

	spanInfo := buildSpanSummary(serviceName, span, resourceAttr, delay)

	stats, ok := p.lateSpanData[bucketDef.name]
	if !ok {
		stats = &lateSpanBucketStats{
			FirstSpan:  spanInfo,
			MinDelay:   delay,
			MinSpan:    spanInfo,
			MaxDelay:   delay,
			MaxSpan:    spanInfo,
			ServiceMap: make(map[string]*serviceSample),
		}
		p.lateSpanData[bucketDef.name] = stats
	}

	stats.Count++

	if stats.Count == 1 {
		stats.FirstSpan = spanInfo
	}

	if delay < stats.MinDelay {
		stats.MinDelay = delay
		stats.MinSpan = spanInfo
	}
	if delay > stats.MaxDelay {
		stats.MaxDelay = delay
		stats.MaxSpan = spanInfo
	}

	serviceStats, ok := stats.ServiceMap[serviceName]
	if !ok {
		serviceStats = &serviceSample{}
		stats.ServiceMap[serviceName] = serviceStats
	}
	serviceStats.Count++
	if len(serviceStats.SampleSpanNames) < maxServiceSamplesPerBucket {
		serviceStats.SampleSpanNames = append(serviceStats.SampleSpanNames, span.Name())
	}
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
		DelayMs:      delay.Milliseconds(),
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

func (p *processorImp) collectAndResetLateSpanData() []lateSpanBucketReport {
	if len(p.lateSpanData) == 0 {
		return nil
	}

	reports := make([]lateSpanBucketReport, 0, len(p.lateSpanData))
	for bucketName, stats := range p.lateSpanData {
		if stats == nil || stats.Count == 0 {
			continue
		}
		report := lateSpanBucketReport{
			Bucket:       bucketName,
			Count:        stats.Count,
			FirstSpan:    stats.FirstSpan,
			MinDelayMs:   stats.MinDelay.Milliseconds(),
			MinSpan:      stats.MinSpan,
			MaxDelayMs:   stats.MaxDelay.Milliseconds(),
			MaxSpan:      stats.MaxSpan,
			ServiceStats: make(map[string]serviceSample, len(stats.ServiceMap)),
		}
		for svc, svcStats := range stats.ServiceMap {
			if svcStats == nil {
				continue
			}
			report.ServiceStats[svc] = serviceSample{
				Count:           svcStats.Count,
				SampleSpanNames: append([]string(nil), svcStats.SampleSpanNames...),
			}
		}
		reports = append(reports, report)
	}

	p.lateSpanData = make(map[string]*lateSpanBucketStats)
	return reports
}

func (p *processorImp) logLateSpanReports(reports []lateSpanBucketReport) {
	for _, report := range reports {
		if report.Count == 0 {
			continue
		}
		p.logger.Warn("Late spans observed before exporting metrics",
			zap.String("bucket", report.Bucket),
			zap.Int("count", report.Count),
			zap.Int64("min_delay_ms", report.MinDelayMs),
			zap.Int64("max_delay_ms", report.MaxDelayMs),
			zap.Any("first_span", report.FirstSpan),
			zap.Any("min_span", report.MinSpan),
			zap.Any("max_span", report.MaxSpan),
			zap.Any("service_stats", report.ServiceStats),
		)
	}
}
