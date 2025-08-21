package v1

import (
	"regexp"
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

	assert.Equal(t, 0, meter.Count(md))
}

func TestMetrics_CountGaugeMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		GaugeMetricsCount:   10,
		GaugeDataPointCount: 10,
	}))

	meter := NewMetrics(zap.NewNop())

	// 10 metrics * 10 data points = 100
	assert.Equal(t, 100, meter.Count(md))
}

func TestMetrics_CountSumMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		SumMetricsCount:   10,
		SumDataPointCount: 6,
	}))

	meter := NewMetrics(zap.NewNop())

	// 10 metrics * 6 data points = 60
	assert.Equal(t, 60, meter.Count(md))
}

func TestMetrics_CountHistogramMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		HistogramMetricsCount:   1,
		HistogramDataPointCount: 6,
		HistogramBucketCount:    20,
	}))

	meter := NewMetrics(zap.NewNop())

	// 6 data points * 20 buckets = 120
	// 6 data points * 4 (sum,count,min,max) metrics = 24
	// total -> 120 + 24 = 144
	assert.Equal(t, 144, meter.Count(md))
}

func TestMetrics_CountExponentialHistogramMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		ExponentialHistogramMetricsCount:   1,
		ExponentialHistogramDataPointCount: 6,
		ExponentialHistogramBucketCount:    20,
	}))

	meter := NewMetrics(zap.NewNop())

	// 6 data points * 20 buckets = 120
	// 120 negative + 120 positive = 240
	// 6 data points * 4 (sum,count,min,max) metrics = 24
	// total -> 240 + 24 = 264
	assert.Equal(t, 264, meter.Count(md))
}

func TestMetrics_CountSummaryMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		SummaryMetricsCount:   1,
		SummaryDataPointCount: 6,
		SummaryQuantileCount:  3,
	}))

	meter := NewMetrics(zap.NewNop())

	assert.Equal(t, 30, meter.Count(md))
}

func TestMetrics_CountSummaryMetrics_WithExcludePattern(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		SummaryMetricsCount:   1,
		SummaryDataPointCount: 6,
		SummaryQuantileCount:  3,
	}))

	excludeRegex = regexp.MustCompile("^zk.duration*")
	meter := NewMetrics(zap.NewNop())

	assert.Equal(t, 0, meter.Count(md))
}
