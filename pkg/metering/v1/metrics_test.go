package v1

import (
	"math"
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

	// 6 data points * (20 buckets + le=+Inf) = 126
	// 6 data points * 4 (sum,count,min,max) metrics = 24
	// total -> 126 + 24 = 150
	assert.Equal(t, 150, meter.Count(md))
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
			expected:     5,
		},
		{
			name:     "NoBuckets",
			expected: 3,
		},
		{
			name:     "BoundsWithoutCounts",
			bounds:   []float64{100, 1000},
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
	md := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		SummaryMetricsCount:   1,
		SummaryDataPointCount: 6,
		SummaryQuantileCount:  3,
	}))

	meter := NewMetrics(zap.NewNop())

	assert.Equal(t, 30, meter.Count(md))
}

func TestMetrics_CountSkipsNaNPoints(t *testing.T) {
	testCases := []struct {
		metricType  pmetric.MetricType
		nanPoints   int
		validPoints int
		expected    int
	}{
		{metricType: pmetric.MetricTypeGauge, nanPoints: 3, validPoints: 4, expected: 4},
		{metricType: pmetric.MetricTypeSum, nanPoints: 3, validPoints: 4, expected: 4},
		// 2 buckets + count + one of sum/min/max per point
		{metricType: pmetric.MetricTypeHistogram, nanPoints: 3, validPoints: 4, expected: 16},
		// 1 quantile + count + sum per point
		{metricType: pmetric.MetricTypeSummary, nanPoints: 2, validPoints: 4, expected: 12},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.metricType.String(), func(t *testing.T) {
			md := generateMetricsWithNaN(testCase.metricType, testCase.nanPoints, testCase.validPoints)
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func generateMetricsWithNaN(metricType pmetric.MetricType, nanPoints, validPoints int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test.metric")

	var numberDps pmetric.NumberDataPointSlice
	switch metricType {
	case pmetric.MetricTypeGauge:
		numberDps = m.SetEmptyGauge().DataPoints()
	case pmetric.MetricTypeSum:
		numberDps = m.SetEmptySum().DataPoints()
	case pmetric.MetricTypeHistogram:
		m.SetEmptyHistogram()
	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
	}

	for i := range nanPoints + validPoints {
		isNaN := i < nanPoints
		value := 1.0
		if isNaN {
			value = math.NaN()
		}

		switch metricType {
		case pmetric.MetricTypeGauge, pmetric.MetricTypeSum:
			dp := numberDps.AppendEmpty()
			if !isNaN && i%2 == 1 {
				dp.SetIntValue(1)
			} else {
				dp.SetDoubleValue(value)
			}
		case pmetric.MetricTypeHistogram:
			dp := m.Histogram().DataPoints().AppendEmpty()
			dp.ExplicitBounds().FromRaw([]float64{1})
			dp.BucketCounts().FromRaw([]uint64{1, 1})
			[]func(float64){dp.SetSum, dp.SetMin, dp.SetMax}[i%3](value)
		case pmetric.MetricTypeSummary:
			dp := m.Summary().DataPoints().AppendEmpty()
			q := dp.QuantileValues().AppendEmpty()
			q.SetValue(1)
			[]func(float64){dp.SetSum, q.SetValue}[i%2](value)
		}
	}

	return md
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
