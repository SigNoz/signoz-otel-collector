// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signozspanmetricsprocessor

import (
	"time"

	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	delta                  = "AGGREGATION_TEMPORALITY_DELTA"
	cumulative             = "AGGREGATION_TEMPORALITY_CUMULATIVE"
	dropSanitizationGateID = "processor.signozspanmetrics.PermissiveLabelSanitization"
)

var dropSanitizationFeatureGate *featuregate.Gate

func init() {
	dropSanitizationFeatureGate = featuregate.GlobalRegistry().MustRegister(
		dropSanitizationGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Controls whether to change labels starting with '_' to 'key_'"),
	)
}

// Dimension defines the dimension name and optional default value if the Dimension is missing from a span attribute.
type Dimension struct {
	Name    string  `mapstructure:"name"`
	Default *string `mapstructure:"default"`
}

// ExcludePattern defines the pattern to exclude from the metrics.
type ExcludePattern struct {
	Name    string `mapstructure:"name"`
	Pattern string `mapstructure:"pattern"`
}

// Config defines the configuration options for spanmetricsprocessor.
type Config struct {

	// MetricsExporter is the name of the metrics exporter to use to ship metrics.
	MetricsExporter string `mapstructure:"metrics_exporter"`

	// LatencyHistogramBuckets is the list of durations representing latency histogram buckets.
	// See defaultLatencyHistogramBucketsMs in processor.go for the default value.
	LatencyHistogramBuckets []time.Duration `mapstructure:"latency_histogram_buckets"`

	// Dimensions defines the list of additional dimensions on top of the provided:
	// - service.name
	// - operation
	// - span.kind
	// - status.code
	// The dimensions will be fetched from the span's attributes. Examples of some conventionally used attributes:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go.
	Dimensions []Dimension `mapstructure:"dimensions"`

	// ExcludePatterns defines the list of patterns to exclude from the metrics.
	ExcludePatterns []ExcludePattern `mapstructure:"exclude_patterns"`

	// DimensionsCacheSize defines the size of cache for storing Dimensions, which helps to avoid cache memory growing
	// indefinitely over the lifetime of the collector.
	// Optional. See defaultDimensionsCacheSize in processor.go for the default value.
	DimensionsCacheSize int `mapstructure:"dimensions_cache_size"`

	AggregationTemporality string `mapstructure:"aggregation_temporality"`

	// skipSanitizeLabel if enabled, labels that start with _ are not sanitized
	skipSanitizeLabel bool

	// MetricsEmitInterval is the time period between when metrics are flushed or emitted to the configured MetricsExporter.
	MetricsFlushInterval time.Duration `mapstructure:"metrics_flush_interval"`

	// TimeBucketInterval is the time interval for bucketing spans based on their start timestamp.
	// Spans are grouped into time buckets based on when they started, not when they are processed.
	// Default is 1 minute.
	TimeBucketInterval time.Duration `mapstructure:"time_bucket_interval"`

	EnableExpHistogram bool `mapstructure:"enable_exp_histogram"`

	MaxServicesToTrack             int `mapstructure:"max_services_to_track"`
	MaxOperationsToTrackPerService int `mapstructure:"max_operations_to_track_per_service"`

	// SkipSpansOlderThan defines the staleness window for skipping late-arriving spans.
	// Spans with start time older than now - SkipSpansOlderThan are skipped.
	// Default is 24 hours if not set.
	SkipSpansOlderThan time.Duration `mapstructure:"skip_spans_older_than"`
}

// GetAggregationTemporality converts the string value given in the config into a AggregationTemporality.
// Returns cumulative, unless delta is correctly specified.
func (c Config) GetAggregationTemporality() pmetric.AggregationTemporality {
	if c.AggregationTemporality == delta {
		return pmetric.AggregationTemporalityDelta
	}
	return pmetric.AggregationTemporalityCumulative
}

// GetTimeBucketInterval returns the configured time bucket interval, or the default if not set.
func (c Config) GetTimeBucketInterval() time.Duration {
	if c.TimeBucketInterval == 0 {
		return defaultTimeBucketInterval
	}
	return c.TimeBucketInterval
}

// GetSkipSpansOlderThan returns the configured staleness window or a default of 24 hours.
func (c Config) GetSkipSpansOlderThan() time.Duration {
	if c.SkipSpansOlderThan <= 0 {
		return defaultSkipSpansOlderThan
	}
	return c.SkipSpansOlderThan
}
