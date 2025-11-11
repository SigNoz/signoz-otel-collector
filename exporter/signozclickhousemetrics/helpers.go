package signozclickhousemetrics

import (
	"time"

	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
)

var lateMetricBuckets = []bucketDefinition{
	{name: "5m-10m", min: 5 * time.Minute, max: 10 * time.Minute},
	{name: "10m-30m", min: 10 * time.Minute, max: 30 * time.Minute},
	{name: "30m-2h", min: 30 * time.Minute, max: 2 * time.Hour},
	{name: "2h-6h", min: 2 * time.Hour, max: 6 * time.Hour},
	{name: "6h-1d", min: 6 * time.Hour, max: 24 * time.Hour},
}

type bucketDefinition struct {
	name string
	min  time.Duration
	max  time.Duration
}

type metricSummary struct {
	MetricName    string            `json:"metric_name"`
	Env           string            `json:"env"`
	Fingerprint   uint64            `json:"fingerprint"`
	Timestamp     string            `json:"timestamp"`
	DelaySeconds  int               `json:"delay_seconds"`
	PrimaryLabels map[string]string `json:"primary_labels,omitempty"`
}

type lateMetricBucketStats struct {
	Count      int
	FirstPoint metricSummary
	MinDelay   time.Duration
	MinPoint   metricSummary
	MaxDelay   time.Duration
	MaxPoint   metricSummary
}

type lateMetricBucketReport struct {
	Bucket          string        `json:"bucket"`
	Count           int           `json:"count"`
	FirstPoint      metricSummary `json:"first_point"`
	MinDelaySeconds int           `json:"min_delay_seconds"`
	MinPoint        metricSummary `json:"min_point"`
	MaxDelaySeconds int           `json:"max_delay_seconds"`
	MaxPoint        metricSummary `json:"max_point"`
}

func bucketForMetricDelay(delay time.Duration) (bucketDefinition, bool) {
	for _, bucket := range lateMetricBuckets {
		if delay >= bucket.min && delay < bucket.max {
			return bucket, true
		}
	}
	return bucketDefinition{}, false
}

func extractPrimaryMetricLabels(resourceMap, fingerprintMap map[string]string) map[string]string {
	primaryMetricLabels := make(map[string]string)
	addIfPresent := func(alias string, source map[string]string, keys ...string) {
		for _, key := range keys {
			if val, ok := source[key]; ok && val != "" {
				primaryMetricLabels[alias] = val
				return
			}
		}
	}
	addIfPresent("service.name", resourceMap, string(semconv.AttributeServiceName))
	addIfPresent("service.namespace", resourceMap, semconv.AttributeServiceNamespace)
	addIfPresent("deployment.environment", resourceMap, semconv.AttributeDeploymentEnvironment)
	addIfPresent("span.kind", fingerprintMap, "span.kind")
	addIfPresent("operation", fingerprintMap, "operation")

	return primaryMetricLabels
}
