package v1

import (
	"regexp"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsTestCase struct {
	name     string
	count    pmetricsgen.Count
	expected int
}

func runMetricsTestCases(t *testing.T, testCases []metricsTestCase) {
	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
		})
	}
}

func TestMetrics(t *testing.T) {
	meter := NewMetrics(zap.NewNop())

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()

	assert.Equal(t, 0, meter.Count(md))
}

func TestMetrics_CountGaugeMetrics(t *testing.T) {
	testCases := []metricsTestCase{
		{
			name:  "AllValid",
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

	runMetricsTestCases(t, testCases)
}

func TestMetrics_CountSumMetrics(t *testing.T) {
	testCases := []metricsTestCase{
		{
			name:  "AllValid",
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

	runMetricsTestCases(t, testCases)
}

func TestMetrics_CountHistogramMetrics(t *testing.T) {
	testCases := []metricsTestCase{
		{
			name:  "AllValid",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramBucketCount: 20},
			// 6 data points * 20 buckets = 120
			// 6 data points * 4 (sum,count,min,max) metrics = 24
			// total -> 120 + 24 = 144
			expected: 144,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramBucketCount: 20, HistogramNanValuesCount: 2},
			// (6 - 2 NaN) data points * 20 buckets = 80
			// (6 - 2 NaN) data points * 4 (sum,count,min,max) metrics = 16
			// total -> 80 + 16 = 96
			expected: 96,
		},
	}

	runMetricsTestCases(t, testCases)
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
	testCases := []metricsTestCase{
		{
			name:  "AllValid",
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

	runMetricsTestCases(t, testCases)
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
