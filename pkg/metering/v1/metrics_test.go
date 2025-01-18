package v1

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestMetrics(t *testing.T) {
	meter := NewMetrics(zap.NewNop())

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()

	assert.Equal(t, 0, meter.Size(md))
}

func TestMetrics_CountGaugeMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.GenerationConfig{
		GaugeMetricsCount:   10,
		GaugeDataPointCount: 10,
	})

	meter := NewMetrics(zap.NewNop())

	// 10 metrics * 10 data points = 100
	assert.Equal(t, 100, meter.Count(md))
}

func TestMetrics_CountSumMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.GenerationConfig{
		SumMetricsCount:   10,
		SumDataPointCount: 6,
	})

	meter := NewMetrics(zap.NewNop())

	// 10 metrics * 6 data points = 60
	assert.Equal(t, 60, meter.Count(md))
}

func TestMetrics_CountHistogramMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.GenerationConfig{
		HistogramMetricsCount:   1,
		HistogramDataPointCount: 6,
		HistogramBucketCount:    20,
	})

	meter := NewMetrics(zap.NewNop())

	// 6 data points * 20 buckets = 120
	assert.Equal(t, 120, meter.Count(md))
}

func TestMetrics_CountExponentialHistogramMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.GenerationConfig{
		ExponentialHistogramMetricsCount:   1,
		ExponentialHistogramDataPointCount: 6,
		ExponentialHistogramBucketCount:    20,
	})

	meter := NewMetrics(zap.NewNop())

	// 6 data points * 20 buckets = 120
	// 120 negative + 120 positive = 240
	assert.Equal(t, 240, meter.Count(md))
}

func TestMetrics_CountSummaryMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.GenerationConfig{
		SummaryMetricsCount:   1,
		SummaryDataPointCount: 6,
		SummaryQuantileCount:  3,
	})

	meter := NewMetrics(zap.NewNop())

	assert.Equal(t, 18, meter.Count(md))
}
