package signozmeterconnector

import (
	"fmt"
	"time"
)

// metric definitions for different telemetry signals
// naming based on the convention specified here - https://opentelemetry.io/docs/specs/semconv/general/naming/#metrics
const (
	metricNameSpansCount = "signoz.meter.span.count"
	metricNameSpansSize  = "signoz.meter.span.size"
	metricDescSpansCount = "The number of spans observed."
	metricDescSpansSize  = "The size of spans observed."

	metricNameMetricsDataPointsCount = "signoz.meter.metric.datapoint.count"
	metricNameMetricsDataPointsSize  = "signoz.meter.metric.datapoint.size"
	metricDescMetricsDataPointsCount = "The number of data points observed."
	metricDescMetricsDataPointsSize  = "The size of data points observed."

	metricNameLogsCount = "signoz.meter.log.count"
	metricNameLogsSize  = "signoz.meter.log.size"
	metricDescLogsCount = "The number of log records observed."
	metricDescLogsSize  = "The size of log records observed."
)

// Config for the connector
type Config struct {
	// the dimensions to record from telemetry data resource attributes for meter metrics labels
	Dimensions []Dimension `mapstructure:"dimensions"`

	// MetricsEmitInterval is the time period between when metrics are flushed or emitted to the configured MetricsExporter.
	MetricsFlushInterval time.Duration `mapstructure:"metrics_flush_interval"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type Dimension struct {
	Key          string  `mapstructure:"key"`
	DefaultValue *string `mapstructure:"default_value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if err := validateDimensions(c.Dimensions); err != nil {
		return fmt.Errorf("failed validating dimensions: %w", err)
	}

	return nil
}

// validateDimensions checks duplicates for dimensions.
func validateDimensions(dimensions []Dimension) error {
	labelNames := make(map[string]struct{})
	for _, key := range dimensions {
		if _, ok := labelNames[key.Key]; ok {
			return fmt.Errorf("duplicate dimension key %s", key.Key)
		}
		labelNames[key.Key] = struct{}{}
	}

	return nil
}
