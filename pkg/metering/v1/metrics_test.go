package v1

import (
	"regexp"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/metricreductionrule"
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
		name           string
		count          pmetricsgen.Count
		metricName     string
		rules          metricreductionrule.RuleSet
		expected       int
		expectedRouted int
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
		{
			name:  "WithRule_Routed",
			count: pmetricsgen.Count{GaugeMetricsCount: 10, GaugeDataPointCount: 10},
			rules: metricreductionrule.RuleSet{"system.memory.usage0": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 10 metrics * 10 data points = 100
			expected: 100,
			// 1 ruled metric * 10 data points = 10 routed
			expectedRouted: 10,
		},
		{
			name:  "WithRule_EffectiveFromMidway_RoutedFromEffectiveFrom",
			count: pmetricsgen.Count{GaugeMetricsCount: 1, GaugeDataPointCount: 10},
			rules: metricreductionrule.RuleSet{"system.memory.usage0": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187000)},
			// 1 metric * 10 data points = 10
			expected: 10,
			// data point j is at 1727286182000 + j*1000 ms; j=5..9 are at or after effective_from = 5 routed
			expectedRouted: 5,
		},
		{
			name:  "WithNaN_WithRule_NaNNotRouted",
			count: pmetricsgen.Count{GaugeMetricsCount: 1, GaugeDataPointCount: 10, GaugeNanValuesCount: 3},
			rules: metricreductionrule.RuleSet{"system.memory.usage0": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 1 metric * (10 - 3 NaN) data points = 7
			expected: 7,
			// (10 - 3 NaN) counted data points = 7 routed
			expectedRouted: 7,
		},
		{
			name:  "WithRuleOnOtherMetric_NotRouted",
			count: pmetricsgen.Count{GaugeMetricsCount: 1, GaugeDataPointCount: 10},
			rules: metricreductionrule.RuleSet{"system.memory.usage1": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 1 metric * 10 data points = 10
			expected: 10,
			// system.memory.usage0 has no rule = 0 routed
			expectedRouted: 0,
		},
		{
			name:       "ExcludedMetric_WithRule_NotCountedNorRouted",
			count:      pmetricsgen.Count{GaugeMetricsCount: 1, GaugeDataPointCount: 10},
			metricName: "signoz.collector.uptime",
			rules:      metricreductionrule.RuleSet{"signoz.collector.uptime": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// signoz* metrics are excluded = 0
			expected: 0,
			// excluded metrics are never routed = 0 routed
			expectedRouted: 0,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			if testCase.metricName != "" {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName(testCase.metricName)
			}
			assert.Equal(t, testCase.expected, meter.Count(md))
			assert.Equal(t, testCase.expectedRouted, meter.CountRoutedPerResource(md.ResourceMetrics().At(0), testCase.rules))
		})
	}
}

func TestMetrics_CountSumMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		count          pmetricsgen.Count
		rules          metricreductionrule.RuleSet
		expected       int
		expectedRouted int
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
		{
			name:  "WithRule_Routed",
			count: pmetricsgen.Count{SumMetricsCount: 10, SumDataPointCount: 6},
			rules: metricreductionrule.RuleSet{"system.cpu.time0": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 10 metrics * 6 data points = 60
			expected: 60,
			// 1 ruled metric * 6 data points = 6 routed
			expectedRouted: 6,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
			assert.Equal(t, testCase.expectedRouted, meter.CountRoutedPerResource(md.ResourceMetrics().At(0), testCase.rules))
		})
	}
}

func TestMetrics_CountHistogramMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		count          pmetricsgen.Count
		rules          metricreductionrule.RuleSet
		expected       int
		expectedRouted int
	}{
		{
			name:  "WithoutNaN",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20},
			// 6 data points * (20 buckets + le=+Inf) = 126
			// 6 data points * 4 (sum,count,min,max) metrics = 24
			// total -> 126 + 24 = 150
			expected: 150,
		},
		{
			name:  "WithNaN",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20, HistogramNanValuesCount: 2},
			// (6 - 2 NaN) data points * (20 buckets + le=+Inf) = 84
			// (6 - 2 NaN) data points * 4 (sum,count,min,max) metrics = 16
			// total -> 84 + 16 = 100
			expected: 100,
		},
		{
			name:  "WithBoundsAndInfBucket",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2, HistogramBucketCount: 3},
			// 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7
			expected: 7,
		},
		{
			name:  "WithOnlyInfBucket",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1},
			// le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 5
			expected: 5,
		},
		{
			name:  "WithBoundsWithoutBucketCounts",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2},
			// bounds without counts aren't written, so le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 5
			expected: 5,
		},
		{
			name:  "WithBucketRule_BucketsRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2, HistogramBucketCount: 3},
			rules: metricreductionrule.RuleSet{"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7
			expected: 7,
			// 2 bounds + le=+Inf (1) = 3 bucket samples routed
			expectedRouted: 3,
		},
		{
			name:  "WithBucketRuleBoundsWithoutBucketCounts_InfBucketRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2},
			rules: metricreductionrule.RuleSet{"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 5
			expected: 5,
			// le=+Inf (1) = 1 bucket sample routed
			expectedRouted: 1,
		},
		{
			name:  "WithCountRule_CountRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20},
			rules: metricreductionrule.RuleSet{"http.server.duration0.count": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 6 data points * (20 buckets + le=+Inf + sum,count,min,max) = 150
			expected: 150,
			// 6 data points * count (1) = 6 routed
			expectedRouted: 6,
		},
		{
			name:  "WithAllRules_Routed",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2, HistogramBucketCount: 3},
			rules: metricreductionrule.RuleSet{
				"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
				"http.server.duration0.count":  metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
				"http.server.duration0.sum":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
				"http.server.duration0.min":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
				"http.server.duration0.max":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
			},
			// 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7
			expected: 7,
			// every sample: 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7 routed
			expectedRouted: 7,
		},
		{
			name:  "WithAllRules_BeforeEffectiveFrom_NotRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20},
			rules: metricreductionrule.RuleSet{
				"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187001),
				"http.server.duration0.count":  metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187001),
				"http.server.duration0.sum":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187001),
				"http.server.duration0.min":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187001),
				"http.server.duration0.max":    metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286187001),
			},
			// 6 data points * (20 buckets + le=+Inf + sum,count,min,max) = 150
			expected: 150,
			// last data point is at 1727286182000 + 5*1000 = 1727286187000 ms, before effective_from = 0 routed
			expectedRouted: 0,
		},
		{
			name:  "WithBucketRule_EffectiveFromMidway_RoutedFromEffectiveFrom",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20},
			rules: metricreductionrule.RuleSet{"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 1727286185000)},
			// 6 data points * (20 buckets + le=+Inf + sum,count,min,max) = 150
			expected: 150,
			// data point j is at 1727286182000 + j*1000 ms; j=3..5 (3) * (20 buckets + le=+Inf (1)) = 63 routed
			expectedRouted: 63,
		},
		{
			name:  "WithNaN_WithSumRule_NaNNotRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 6, HistogramExplicitBoundsCount: 20, HistogramBucketCount: 20, HistogramNanValuesCount: 2},
			rules: metricreductionrule.RuleSet{"http.server.duration0.sum": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// (6 - 2 NaN) data points * (20 buckets + le=+Inf + sum,count,min,max) = 100
			expected: 100,
			// (6 - 2 NaN) data points * sum (1) = 4 routed
			expectedRouted: 4,
		},
		{
			name:  "WithRuleOnUnflattenedName_NotRouted",
			count: pmetricsgen.Count{HistogramMetricsCount: 1, HistogramDataPointCount: 1, HistogramExplicitBoundsCount: 2, HistogramBucketCount: 3},
			rules: metricreductionrule.RuleSet{"http.server.duration0": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7
			expected: 7,
			// rules match flattened names only = 0 routed
			expectedRouted: 0,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
			assert.Equal(t, testCase.expectedRouted, meter.CountRoutedPerResource(md.ResourceMetrics().At(0), testCase.rules))
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
	routed := meter.CountRoutedPerResource(md.ResourceMetrics().At(0), metricreductionrule.RuleSet{
		"http.server.duration0":        metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
		"http.server.duration0.bucket": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
		"http.server.duration0.count":  metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
	})
	assert.Equal(t, 0, routed)
}

func TestMetrics_CountSummaryMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		count          pmetricsgen.Count
		rules          metricreductionrule.RuleSet
		expected       int
		expectedRouted int
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
		{
			name:  "WithQuantileRule_QuantilesRouted",
			count: pmetricsgen.Count{SummaryMetricsCount: 1, SummaryDataPointCount: 6, SummaryQuantileCount: 3},
			rules: metricreductionrule.RuleSet{"zk.duration0.quantile": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0)},
			// 6 data points * (3 quantiles + count (1) + sum (1)) = 30
			expected: 30,
			// 6 data points * 3 quantiles = 18 routed
			expectedRouted: 18,
		},
		{
			name:  "WithNaN_WithCountAndSumRules_NaNNotRouted",
			count: pmetricsgen.Count{SummaryMetricsCount: 1, SummaryDataPointCount: 6, SummaryQuantileCount: 3, SummaryNanValuesCount: 2},
			rules: metricreductionrule.RuleSet{
				"zk.duration0.count": metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
				"zk.duration0.sum":   metricreductionrule.NewReductionRule(map[string]struct{}{"pod": {}}, false, 0),
			},
			// (6 - 2 NaN) data points * (3 quantiles + count (1) + sum (1)) = 20
			expected: 20,
			// (6 - 2 NaN) data points * (count (1) + sum (1)) = 8 routed
			expectedRouted: 8,
		},
	}

	meter := NewMetrics(zap.NewNop())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md := pmetricsgen.Generate(pmetricsgen.WithCount(testCase.count))
			assert.Equal(t, testCase.expected, meter.Count(md))
			assert.Equal(t, testCase.expectedRouted, meter.CountRoutedPerResource(md.ResourceMetrics().At(0), testCase.rules))
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
