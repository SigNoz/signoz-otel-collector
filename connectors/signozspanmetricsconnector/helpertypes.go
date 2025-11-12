package signozspanmetricsconnector

import (
	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type metricKey string

type dimension struct {
	name  string
	value *pcommon.Value
}

type histogramData struct {
	count         uint64
	sum           float64
	bucketCounts  []uint64
	exemplarsData []exemplarData
}

type exemplarData struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID
	value   float64
}

type exponentialHistogram struct {
	histogram *structure.Histogram[float64]
}

func (h *exponentialHistogram) Observe(value float64) {
	h.histogram.Update(value)
}

// Dimension defines the dimension name and optional default value if the Dimension is missing from a span attribute.
type Dimension struct {
	Name    string  `mapstructure:"name"`
	Default *string `mapstructure:"default"`
}

// ExcludePattern defines the pattern to exclude from the metrics.
type ExcludePattern struct {
	// Name is the name of the pattern.
	Name string `mapstructure:"name"`
	// Pattern is the pattern to exclude from the metrics.
	Pattern string `mapstructure:"pattern"`
}
