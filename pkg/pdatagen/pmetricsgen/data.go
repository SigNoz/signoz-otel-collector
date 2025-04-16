package pmetricsgen

import (
	"github.com/prometheus/prometheus/model/value"
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Generate(opts ...GenerationOption) pmetric.Metrics {
	generationOpts := generationOptions{
		resourceAttributeCount:       1,
		resourceAttributeStringValue: "resource",
		scopeAttributeCount:          1,
		scopeAttributeStringValue:    "scope",
		count:                        Count{},
		attributes:                   map[string]any{},
	}

	for _, opt := range opts {
		opt(&generationOpts)
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < generationOpts.resourceAttributeCount; i++ {
		suffix := strconv.Itoa(i)
		// Do not change the key name format in resource attributes below.
		resourceMetrics.Resource().Attributes().PutStr("resource."+suffix, generationOpts.resourceAttributeStringValue)
	}

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	for i := 0; i < generationOpts.scopeAttributeCount; i++ {
		suffix := strconv.Itoa(i)
		// Do not change the key name format in resource attributes below.
		scopeMetrics.Scope().Attributes().PutStr("scope."+suffix, generationOpts.scopeAttributeStringValue)
	}

	scopeMetrics.Metrics().EnsureCapacity(
		generationOpts.count.GaugeMetricsCount +
			generationOpts.count.SumMetricsCount +
			generationOpts.count.HistogramMetricsCount +
			generationOpts.count.ExponentialHistogramMetricsCount,
	)

	for i := 0; i < generationOpts.count.GaugeMetricsCount; i++ {
		gaugeMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyGauge()
		gaugeMetric.DataPoints().EnsureCapacity(generationOpts.count.GaugeDataPointCount)
		for j := 0; j < generationOpts.count.GaugeDataPointCount; j++ {
			dp := gaugeMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetDoubleValue(rand.Float64())
		}
	}

	for i := 0; i < generationOpts.count.SumMetricsCount; i++ {
		sumMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptySum()
		sumMetric.DataPoints().EnsureCapacity(generationOpts.count.SumDataPointCount)
		for j := 0; j < generationOpts.count.SumDataPointCount; j++ {
			dp := sumMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetDoubleValue(rand.Float64())
		}
	}

	for i := 0; i < generationOpts.count.HistogramMetricsCount; i++ {
		histogramMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyHistogram()
		histogramMetric.DataPoints().EnsureCapacity(generationOpts.count.HistogramDataPointCount)
		for j := 0; j < generationOpts.count.HistogramDataPointCount; j++ {
			dp := histogramMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			bucketsCounts := dp.BucketCounts()
			bucketsCounts.EnsureCapacity(generationOpts.count.HistogramBucketCount)
			for k := 0; k < generationOpts.count.HistogramBucketCount; k++ {
				bucketsCounts.Append(rand.Uint64())
			}
			dp.ExplicitBounds().EnsureCapacity(generationOpts.count.HistogramBucketCount)
			for k := 0; k < generationOpts.count.HistogramBucketCount; k++ {
				dp.ExplicitBounds().Append(rand.Float64())
			}
		}
	}

	for i := 0; i < generationOpts.count.ExponentialHistogramMetricsCount; i++ {
		exponentialHistogramMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptyExponentialHistogram()
		exponentialHistogramMetric.DataPoints().EnsureCapacity(generationOpts.count.ExponentialHistogramDataPointCount)
		for j := 0; j < generationOpts.count.ExponentialHistogramDataPointCount; j++ {
			dp := exponentialHistogramMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetCount(rand.Uint64())
			dp.SetSum(rand.Float64())
			dp.SetMin(rand.Float64())
			dp.SetMax(rand.Float64())
			negative := dp.Negative().BucketCounts()
			negative.EnsureCapacity(generationOpts.count.ExponentialHistogramBucketCount)
			for k := 0; k < generationOpts.count.ExponentialHistogramBucketCount; k++ {
				negative.Append(rand.Uint64())
			}
			positive := dp.Positive().BucketCounts()
			positive.EnsureCapacity(generationOpts.count.ExponentialHistogramBucketCount)
			for k := 0; k < generationOpts.count.ExponentialHistogramBucketCount; k++ {
				positive.Append(rand.Uint64())
			}
		}
	}

	for i := 0; i < generationOpts.count.SummaryMetricsCount; i++ {
		summaryMetric := scopeMetrics.Metrics().AppendEmpty().SetEmptySummary()
		summaryMetric.DataPoints().EnsureCapacity(generationOpts.count.SummaryDataPointCount)
		for j := 0; j < generationOpts.count.SummaryDataPointCount; j++ {
			dp := summaryMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			quantiles := dp.QuantileValues()
			quantiles.EnsureCapacity(generationOpts.count.SummaryQuantileCount)
			for k := 0; k < generationOpts.count.SummaryQuantileCount; k++ {
				quantiles.AppendEmpty().SetQuantile(rand.Float64())
			}
			dp.SetCount(rand.Uint64())
			dp.SetSum(rand.Float64())
		}
	}

	return metrics
}

// GenerateGaugeMetrics generates a set of gauge metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as system.memory.usage0, system.memory.usage1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as gauge.attr0, gauge.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func GenerateGaugeMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.memory.usage" + strconv.Itoa(i))
		m.SetUnit("bytes")
		m.SetDescription("memory usage of the host")
		dpSlice := m.SetEmptyGauge().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
	}
	return metrics
}

// GenerateSumMetrics generates a set of sum metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as system.cpu.time0, system.cpu.time1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as sum.attr0, sum.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
// the sum metrics will be cumulative or delta based on the index of the metric
// for even metrics i.e system.cpu.time0, system.cpu.time2, etc will be cumulative
// for odd metrics i.e system.cpu.time1, system.cpu.time3, etc will be delta
// the sum metrics will be monotonic or not based on the index of the metric
// for even metrics i.e system.cpu.time0, system.cpu.time2, etc will be monotonic
// for odd metrics i.e system.cpu.time1, system.cpu.time3, etc will be not monotonic
func GenerateSumMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.cpu.time" + strconv.Itoa(i))
		m.SetUnit("s")
		m.SetDescription("cpu time of the host")
		dpSlice := m.SetEmptySum().DataPoints()
		if i%2 == 0 {
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		if i%3 == 0 {
			m.Sum().SetIsMonotonic(true)
		} else {
			m.Sum().SetIsMonotonic(false)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("sum.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
	}
	return metrics
}

// GenerateHistogramMetrics generates a set of histogram metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as http.server.duration0, http.server.duration1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as histogram.attr0, histogram.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
// the histogram metrics will be cumulative or delta based on the index of the metric
// for even metrics i.e http.server.duration0, http.server.duration2, etc will be cumulative
// for odd metrics i.e http.server.duration1, http.server.duration3, etc will be delta
// the default number of buckets is 20
// the default bucket counts are `1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1`
// the default explicit bounds are `0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19`
func GenerateHistogramMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server")
		dpSlice := m.SetEmptyHistogram().DataPoints()
		if i%2 == 0 {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetCount(30)
			dp.SetSum(35)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.ExplicitBounds().FromRaw([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
			dp.BucketCounts().FromRaw([]uint64{1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1})
			dp.SetMin(0)
			dp.SetMax(12)
		}
	}
	return metrics
}

// GenerateExponentialHistogramMetrics generates a set of exponential histogram metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as http.server.duration0, http.server.duration1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as exponential.histogram.attr0, exponential.histogram.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
// the exponential histogram metrics will be cumulative or delta based on the index of the metric
// for even metrics i.e http.server.duration0, http.server.duration2, etc will be cumulative
// for odd metrics i.e http.server.duration1, http.server.duration3, etc will be delta
func GenerateExponentialHistogramMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server but in exponential histogram format")
		dpSlice := m.SetEmptyExponentialHistogram().DataPoints()
		if i%2 == 0 {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetSum(1)
			dp.SetMin(0)
			dp.SetMax(1)
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
		}
	}
	return metrics
}

// GenerateSummaryMetrics generates a set of summary metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as zk.duration0, zk.duration1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as summary.attr0, summary.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func GenerateSummaryMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("zk.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		slice := m.SetEmptySummary().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(float64(j))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			quantileValues := dp.QuantileValues().AppendEmpty()
			quantileValues.SetValue(float64(j))
			quantileValues.SetQuantile(float64(j))
		}
	}
	return metrics
}

func GenerateGaugeNanMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.memory.usage" + strconv.Itoa(i))
		m.SetUnit("bytes")
		m.SetDescription("memory usage of the host")
		dpSlice := m.SetEmptyGauge().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetDoubleValue(math.NaN())
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
	}
	return metrics
}

func GenerateGaugeMetricsWithNoRecordedValueFlag(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.memory.usage" + strconv.Itoa(i))
		m.SetUnit("bytes")
		m.SetDescription("memory usage of the host")
		dpSlice := m.SetEmptyGauge().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			dp.SetDoubleValue(float64(i))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
	}
	return metrics
}

func GenerateHistogramMetricsWithNoRecordedValueFlag(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server")
		dpSlice := m.SetEmptyHistogram().DataPoints()
		if i%2 == 0 {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			dp.SetCount(30)
			dp.SetSum(35)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.ExplicitBounds().FromRaw([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
			dp.BucketCounts().FromRaw([]uint64{1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1})
			dp.SetMin(0)
			dp.SetMax(12)
		}
	}
	return metrics
}

func GenerateHistogramMetricsWithNanValuesAndNilValues(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server")
		dpSlice := m.SetEmptyHistogram().DataPoints()
		if i%2 == 0 {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			dp.SetCount(30)
			dp.SetSum(math.Float64frombits(value.StaleNaN))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.ExplicitBounds().FromRaw([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
			dp.BucketCounts().FromRaw([]uint64{1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1})
			dp.SetMin(math.Float64frombits(value.StaleNaN))
			dp.SetMax(12)
		}
	}
	return metrics
}

func GenerateSumMetricsWithNoRecordedValue(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.cpu.time" + strconv.Itoa(i))
		m.SetUnit("s")
		m.SetDescription("cpu time of the host")
		dpSlice := m.SetEmptySum().DataPoints()
		if i%2 == 0 {
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		if i%3 == 0 {
			m.Sum().SetIsMonotonic(true)
		} else {
			m.Sum().SetIsMonotonic(false)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("sum.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
	}
	return metrics
}

func GenerateSummaryMetricsWithNoRecordedValue(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("zk.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		slice := m.SetEmptySummary().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(float64(j))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			quantileValues := dp.QuantileValues().AppendEmpty()
			quantileValues.SetValue(float64(j))
			quantileValues.SetQuantile(float64(j))
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}
	}
	return metrics
}

func GenerateSummaryMetricsWithNan(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("zk.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		slice := m.SetEmptySummary().DataPoints()
		for j := 0; j < numDataPoints; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(float64(j))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			quantileValues := dp.QuantileValues().AppendEmpty()
			quantileValues.SetValue(math.NaN())
			quantileValues.SetQuantile(float64(j))
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}
	}
	return metrics
}

func GenerateExponentialHistogramMetricsWithNoRecordedValue(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server but in exponential histogram format")
		dpSlice := m.SetEmptyExponentialHistogram().DataPoints()
		if i%2 == 0 {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetSum(1)
			dp.SetMin(0)
			dp.SetMax(1)
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}
	}
	return metrics
}

func GenerateExponentialHistogramMetricsWithNan(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttributes; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttributes; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), "value"+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server but in exponential histogram format")
		dpSlice := m.SetEmptyExponentialHistogram().DataPoints()
		if i%2 == 0 {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetSum(math.NaN())
			dp.SetMin(0)
			dp.SetMax(math.NaN())
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10})
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}
	}
	return metrics
}
