package pmetricsgen

import (
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type GenerationConfig struct {
	GaugeMetricsCount                  int
	GaugeDataPointCount                int
	SumMetricsCount                    int
	SumDataPointCount                  int
	HistogramMetricsCount              int
	HistogramDataPointCount            int
	HistogramBucketCount               int
	ExponentialHistogramMetricsCount   int
	ExponentialHistogramDataPointCount int
	ExponentialHistogramBucketCount    int
	SummaryMetricsCount                int
	SummaryDataPointCount              int
	SummaryQuantileCount               int
}

func Generate(cfg GenerationConfig) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Metrics().EnsureCapacity(
		cfg.GaugeMetricsCount +
			cfg.SumMetricsCount +
			cfg.HistogramMetricsCount +
			cfg.ExponentialHistogramMetricsCount,
	)

	for i := 0; i < cfg.GaugeMetricsCount; i++ {
		gaugeMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyGauge()
		gaugeMetric.DataPoints().EnsureCapacity(cfg.GaugeDataPointCount)
		for j := 0; j < cfg.GaugeDataPointCount; j++ {
			dp := gaugeMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetDoubleValue(rand.Float64())
		}
	}

	for i := 0; i < cfg.SumMetricsCount; i++ {
		sumMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptySum()
		sumMetric.DataPoints().EnsureCapacity(cfg.SumDataPointCount)
		for j := 0; j < cfg.SumDataPointCount; j++ {
			dp := sumMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetDoubleValue(rand.Float64())
		}
	}

	for i := 0; i < cfg.HistogramMetricsCount; i++ {
		histogramMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyHistogram()
		histogramMetric.DataPoints().EnsureCapacity(cfg.HistogramDataPointCount)
		for j := 0; j < cfg.HistogramDataPointCount; j++ {
			dp := histogramMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			bucketsCounts := dp.BucketCounts()
			bucketsCounts.EnsureCapacity(cfg.HistogramBucketCount)
			for k := 0; k < cfg.HistogramBucketCount; k++ {
				bucketsCounts.Append(rand.Uint64())
			}
			dp.ExplicitBounds().EnsureCapacity(cfg.HistogramBucketCount)
			for k := 0; k < cfg.HistogramBucketCount; k++ {
				dp.ExplicitBounds().Append(rand.Float64())
			}
		}
	}

	for i := 0; i < cfg.ExponentialHistogramMetricsCount; i++ {
		exponentialHistogramMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyExponentialHistogram()
		exponentialHistogramMetric.DataPoints().EnsureCapacity(cfg.ExponentialHistogramDataPointCount)
		for j := 0; j < cfg.ExponentialHistogramDataPointCount; j++ {
			dp := exponentialHistogramMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetCount(rand.Uint64())
			dp.SetSum(rand.Float64())
			dp.SetMin(rand.Float64())
			dp.SetMax(rand.Float64())
			negative := dp.Negative().BucketCounts()
			negative.EnsureCapacity(cfg.ExponentialHistogramBucketCount)
			for k := 0; k < cfg.ExponentialHistogramBucketCount; k++ {
				negative.Append(rand.Uint64())
			}
			positive := dp.Positive().BucketCounts()
			positive.EnsureCapacity(cfg.ExponentialHistogramBucketCount)
			for k := 0; k < cfg.ExponentialHistogramBucketCount; k++ {
				positive.Append(rand.Uint64())
			}
		}
	}

	for i := 0; i < cfg.SummaryMetricsCount; i++ {
		summaryMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptySummary()
		summaryMetric.DataPoints().EnsureCapacity(cfg.SummaryDataPointCount)
		for j := 0; j < cfg.SummaryDataPointCount; j++ {
			dp := summaryMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			quantiles := dp.QuantileValues()
			quantiles.EnsureCapacity(cfg.SummaryQuantileCount)
			for k := 0; k < cfg.SummaryQuantileCount; k++ {
				quantiles.AppendEmpty().SetQuantile(rand.Float64())
			}
			dp.SetCount(rand.Uint64())
			dp.SetSum(rand.Float64())
		}
	}

	return metrics
}
