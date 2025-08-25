package signozmeterconnector

import (
	"fmt"
	"time"
)

// metric definitions for different telemetry signals
// naming based on the convention specified here - https://opentelemetry.io/docs/specs/semconv/general/naming/#metrics
const (
	metricNameSpansCount = "signoz.meter.span.count"
	metricDescSpansCount = "The number of spans observed."
	metricNameSpansSize  = "signoz.meter.span.size"
	metricDescSpansSize  = "The size of spans observed."

	metricNameMetricsDataPointsCount = "signoz.meter.metric.datapoint.count"
	metricDescMetricsDataPointsCount = "The number of data points observed."
	metricNameMetricsDataPointsSize  = "signoz.meter.metric.datapoint.size"
	metricDescMetricsDataPointsSize  = "The size of data points observed."

	metricNameLogsCount = "signoz.meter.log.count"
	metricDescLogsCount = "The number of log records observed."
	metricNameLogsSize  = "signoz.meter.log.size"
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

// Dimension defines the dimension name and optional default value in case the dimension is missing for corresponding telemetry signal's resource attributes
type Dimension struct {
	Name string `mapstructure:"name"`

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
	for _, dimension := range dimensions {
		if _, ok := labelNames[dimension.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", dimension.Name)
		}
		labelNames[dimension.Name] = struct{}{}
	}

	return nil
}
