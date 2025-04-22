package pmetricsgen

import (
	"math"
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
			generationOpts.count.ExponentialHistogramMetricsCount +
			generationOpts.count.SummaryMetricsCount,
	)

	// Use GenerateGaugeMetrics and extract metrics directly
	gaugeMetrics := GenerateGaugeMetrics(
		generationOpts.count.GaugeMetricsCount,
		generationOpts.count.GaugeDataPointCount,
		0, // No point attributes
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.GaugeNanValuesCount, // No NaN values
		generationOpts.count.GaugeNoRecordCount,  // No no-recorded values
	)

	// Extract metrics directly from the generated metrics
	if gaugeMetrics.ResourceMetrics().Len() > 0 &&
		gaugeMetrics.ResourceMetrics().At(0).ScopeMetrics().Len() > 0 {
		sourceMetrics := gaugeMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < sourceMetrics.Len(); i++ {
			metric := sourceMetrics.At(i)
			newMetric := scopeMetrics.Metrics().AppendEmpty()
			metric.CopyTo(newMetric)
		}
	}

	// Use GenerateSumMetrics and extract metrics directly
	sumMetrics := GenerateSumMetrics(
		generationOpts.count.SumMetricsCount,
		generationOpts.count.SumDataPointCount,
		0, // No point attributes
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.SumNoRecordCount,
	)

	// Extract metrics directly from the generated metrics
	if sumMetrics.ResourceMetrics().Len() > 0 &&
		sumMetrics.ResourceMetrics().At(0).ScopeMetrics().Len() > 0 {
		sourceMetrics := sumMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < sourceMetrics.Len(); i++ {
			metric := sourceMetrics.At(i)
			newMetric := scopeMetrics.Metrics().AppendEmpty()
			metric.CopyTo(newMetric)
		}
	}

	// Use GenerateHistogramMetrics and extract metrics directly
	histogramMetrics := GenerateHistogramMetrics(
		generationOpts.count.HistogramMetricsCount,
		generationOpts.count.HistogramDataPointCount,
		0, // No point attributes
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.HistogramNanValuesCount,
		generationOpts.count.HistogramNoRecordCount,
	)

	// Extract metrics directly from the generated metrics
	if histogramMetrics.ResourceMetrics().Len() > 0 &&
		histogramMetrics.ResourceMetrics().At(0).ScopeMetrics().Len() > 0 {
		sourceMetrics := histogramMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < sourceMetrics.Len(); i++ {
			metric := sourceMetrics.At(i)
			newMetric := scopeMetrics.Metrics().AppendEmpty()
			metric.CopyTo(newMetric)
		}
	}

	// Use GenerateExponentialHistogramMetrics and extract metrics directly
	exponentialHistogramMetrics := GenerateExponentialHistogramMetrics(
		generationOpts.count.ExponentialHistogramMetricsCount,
		generationOpts.count.ExponentialHistogramDataPointCount,
		0, // No point attributes
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.ExponentialHistogramBucketCount,
		generationOpts.count.ExponentialHistogramNanValuesCount,
		generationOpts.count.ExponentialHistogramNoRecordCount,
	)

	// Copy the exponential histogram metrics to the scope metrics
	for i := 0; i < exponentialHistogramMetrics.ResourceMetrics().Len(); i++ {
		rm := exponentialHistogramMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				newMetric := scopeMetrics.Metrics().AppendEmpty()
				metric.CopyTo(newMetric)
			}
		}
	}

	// Use GenerateSummaryMetrics and extract metrics directly
	summaryMetrics := GenerateSummaryMetrics(
		generationOpts.count.SummaryMetricsCount,
		generationOpts.count.SummaryDataPointCount,
		0, // No point attributes
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.SummaryQuantileCount,
		generationOpts.count.SummaryNanValuesCount,
		generationOpts.count.SummaryNoRecordCount,
	)

	// Extract metrics directly from the generated metrics
	if summaryMetrics.ResourceMetrics().Len() > 0 &&
		summaryMetrics.ResourceMetrics().At(0).ScopeMetrics().Len() > 0 {
		sourceMetrics := summaryMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < sourceMetrics.Len(); i++ {
			metric := sourceMetrics.At(i)
			newMetric := scopeMetrics.Metrics().AppendEmpty()
			metric.CopyTo(newMetric)
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
func GenerateGaugeMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
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
		validDataPoints := numDataPoints - numNanValues - numNoRecordedValues
		for j := 0; j < validDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
		for j := 0; j < numNoRecordedValues; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
		for j := 0; j < numNanValues; j++ {
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
func GenerateSumMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNoRecordedValue int) pmetric.Metrics {
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
		validPoints := numDataPoints - numNoRecordedValue
		for j := 0; j < validPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("sum.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
		}
		for j := 0; j < numNoRecordedValue; j++ {
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
func GenerateHistogramMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
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
		validDataPoints := numDataPoints - numNanValues - numNoRecordedValues
		for j := 0; j < validDataPoints; j++ {
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

		for j := 0; j < numNanValues; j++ {
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
			dp.SetMin(math.NaN())
			dp.SetMax(math.NaN())
		}

		for j := 0; j < numNoRecordedValues; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetCount(30)
			dp.SetSum(35)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
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
func GenerateExponentialHistogramMetrics(
	numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes, numBucketCount int, numNanValues int, numNoRecordedValues int,
) pmetric.Metrics {
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

	fixedPattern := []uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10}
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

		validDataPoints := numDataPoints - numNanValues - numNoRecordedValues

		for j := 0; j < validDataPoints; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			t := time.Unix(1727286182+int64(j), 0)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetSum(1)
			dp.SetMin(0)
			dp.SetMax(1)
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}

			buckets := make([]uint64, numBucketCount)
			copy(buckets, fixedPattern)

			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw(buckets)

			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw(buckets)
		}

		for j := 0; j < numNanValues; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			t := time.Unix(1727286182+int64(j), 0)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetSum(math.NaN())
			dp.SetMin(math.NaN())
			dp.SetMax(1)
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}

			buckets := make([]uint64, numBucketCount)
			copy(buckets, fixedPattern)

			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw(buckets)

			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw(buckets)
		}

		for j := 0; j < numNoRecordedValues; j++ {
			dp := dpSlice.AppendEmpty()
			dp.SetScale(2)
			t := time.Unix(1727286182+int64(j), 0)
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(t))
			dp.SetSum(1)
			dp.SetMin(0)
			dp.SetMax(1)
			dp.SetZeroCount(0)
			dp.SetCount(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("exponential.histogram.attr_"+strconv.Itoa(k), "1")
			}

			buckets := make([]uint64, numBucketCount)
			copy(buckets, fixedPattern)

			dp.Negative().SetOffset(1)
			dp.Negative().BucketCounts().FromRaw(buckets)

			dp.Positive().SetOffset(1)
			dp.Positive().BucketCounts().FromRaw(buckets)
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
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
func GenerateSummaryMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes, numQuantileCount int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
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
		validDataPoints := numDataPoints - numNanValues - numNoRecordedValues
		for j := 0; j < validDataPoints; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(float64(j))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			for q := 0; q < numQuantileCount; q++ {
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetQuantile(float64(q) / float64(numQuantileCount))
				qv.SetValue(float64(j + q)) // Example: increasing value
			}
		}
		for j := 0; j < numNanValues; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(math.NaN())
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			for q := 0; q < numQuantileCount; q++ {
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetQuantile(float64(q) / float64(numQuantileCount))
				qv.SetValue(float64(j + q)) // Example: increasing value
			}
		}
		for j := 0; j < numNoRecordedValues; j++ {
			dp := slice.AppendEmpty()
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182+int64(j), 0)))
			dp.SetCount(uint64(j))
			dp.SetSum(float64(j))
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			for q := 0; q < numQuantileCount; q++ {
				qv := dp.QuantileValues().AppendEmpty()
				qv.SetQuantile(float64(q) / float64(numQuantileCount))
				qv.SetValue(float64(j + q)) // Example: increasing value
			}
		}
	}
	return metrics
}
