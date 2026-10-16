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
	testCases := []struct {
		name     string
		count    pmetricsgen.Count
		expected int
	}{
		{
			name:  "WithoutNaN",
			count: pmetricsgen.Count{GaugeMetricsCount: 10, GaugeDataPointCount: 10},
			// 10 metrics * 10 data points = 100
			expected: 100,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{GaugeMetricsCount: 10, GaugeDataPointCount: 10, GaugeNanValuesCount: 3},
			// 10 metrics * (10 - 3 NaN) data points = 70
			expected: 70,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func TestMetrics_CountSumMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		count    pmetricsgen.Count
		expected int
	}{
		{
			name:  "WithoutNaN",
			count: pmetricsgen.Count{SumMetricsCount: 10, SumDataPointCount: 6},
			// 10 metrics * 6 data points = 60
			expected: 60,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{SumMetricsCount: 10, SumDataPointCount: 6, SumNanValuesCount: 2},
			// 10 metrics * (6 - 2 NaN) data points = 40
			expected: 40,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func TestMetrics_CountHistogramMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		count    pmetricsgen.Count
		expected int
	}{
		{
			name:  "WithoutNaN",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramBucketCount: 20},
			// 6 data points * (20 buckets + le=+Inf) = 126
			// 6 data points * 4 (sum,count,min,max) metrics = 24
			// total -> 126 + 24 = 150
			expected: 150,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramBucketCount: 20, HistogramNanValuesCount: 2},
			// (6 - 2 NaN) data points * (20 buckets + le=+Inf) = 84
			// (6 - 2 NaN) data points * 4 (sum,count,min,max) metrics = 16
			// total -> 84 + 16 = 100
			expected: 100,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func TestMetrics_CountHistogramBuckets(t *testing.T) {
	testCases := []struct {
		name         string
		bounds       []float64
		bucketCounts []uint64
		expected     int
	}{
		{
			name:         "WellFormed",
			bounds:       []float64{0.5, 1},
			bucketCounts: []uint64{1, 1, 1},
			// 2 bounds + le=+Inf + count (1) + sum (1) = 5
			expected: 5,
		},
		{
			name: "NoBuckets",
			// le=+Inf + count (1) + sum (1) = 3
			expected: 3,
		},
		{
			name:   "BoundsWithoutCounts",
			bounds: []float64{100, 1000},
			// bounds without counts aren't written, so le=+Inf + count (1) + sum (1) = 3
			expected: 3,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetric.NewMetrics()
			m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			m.SetName("db.query.duration")
			dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
			dp.SetCount(3)
			dp.SetSum(1.5)
			dp.ExplicitBounds().FromRaw(testCase.bounds)
			dp.BucketCounts().FromRaw(testCase.bucketCounts)

			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func TestMetrics_CountExponentialHistogramMetrics(t *testing.T) {
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		ExponentialHistogramMetricsCount:   1,
		ExponentialHistogramDataPointCount: 6,
		ExponentialHistogramBucketCount:    20,
	}))

	meter := NewMetrics(zap.NewNop())

	// exponential histograms are intentionally not counted
	assert.Equal(t, 0, meter.Count(md))
}

func TestMetrics_CountSummaryMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		count    pmetricsgen.Count
		expected int
	}{
		{
			name:  "WithoutNaN",
			count: pmetricsgen.Count{SummaryMetricsCount: 1, SummaryDataPointCount: 6, SummaryQuantileCount: 3},
			// 6 data points * (3 quantiles + count (1) + sum (1)) = 30
			expected: 30,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{SummaryMetricsCount: 1, SummaryDataPointCount: 6, SummaryQuantileCount: 3, SummaryNanValuesCount: 2},
			// (6 - 2 NaN) data points * (3 quantiles + count (1) + sum (1)) = 20
			expected: 20,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
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
