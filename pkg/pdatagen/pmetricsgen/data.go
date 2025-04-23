package pmetricsgen

import (
	"math"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Common function to set up resource and scope attributes
func setupResourceAndScope(metrics *pmetric.Metrics, numResourceAttrs, numScopeAttrs int, resourceValue, scopeValue string) (*pmetric.ResourceMetrics, *pmetric.ScopeMetrics) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	for i := 0; i < numResourceAttrs; i++ {
		rm.Resource().Attributes().PutStr("resource.attr_"+strconv.Itoa(i), resourceValue+strconv.Itoa(i))
	}
	rm.SetSchemaUrl("resource.schema_url")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("scope.schema_url")
	for i := 0; i < numScopeAttrs; i++ {
		sm.Scope().Attributes().PutStr("scope.attr_"+strconv.Itoa(i), scopeValue+strconv.Itoa(i))
	}
	sm.Scope().SetName("go.signoz.io/app/reader")
	sm.Scope().SetVersion("1.0.0")

	return &rm, &sm
}

// Helper to set timestamps
func setTimestamps(dp pmetric.NumberDataPoint, baseTime int64, offset int64) {
	timestamp := pcommon.NewTimestampFromTime(time.Unix(baseTime+offset, 0))
	dp.SetStartTimestamp(timestamp)
	dp.SetTimestamp(timestamp)
}

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
	_, sm := setupResourceAndScope(&metrics,
		generationOpts.resourceAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeStringValue,
		generationOpts.scopeAttributeStringValue)

	// Pre-allocate capacity for all metrics
	totalMetrics := generationOpts.count.GaugeMetricsCount +
		generationOpts.count.SumMetricsCount +
		generationOpts.count.HistogramMetricsCount +
		generationOpts.count.ExponentialHistogramMetricsCount +
		generationOpts.count.SummaryMetricsCount
	sm.Metrics().EnsureCapacity(totalMetrics)

	// Generate and copy metrics
	copyMetricsToScope(GenerateGaugeMetrics(
		generationOpts.count.GaugeMetricsCount,
		generationOpts.count.GaugeDataPointCount,
		generationOpts.count.GaugePointAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.GaugeNanValuesCount,
		generationOpts.count.GaugeNoRecordCount,
	), sm)

	copyMetricsToScope(GenerateSumMetrics(
		generationOpts.count.SumMetricsCount,
		generationOpts.count.SumDataPointCount,
		generationOpts.count.SumPointAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.SumNoRecordCount,
		generationOpts.count.SumNanValuesCount,
	), sm)

	copyMetricsToScope(GenerateHistogramMetrics(
		generationOpts.count.HistogramMetricsCount,
		generationOpts.count.HistogramDataPointCount,
		generationOpts.count.HistogramPointAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.HistogramNanValuesCount,
		generationOpts.count.HistogramNoRecordCount,
	), sm)

	copyMetricsToScope(GenerateExponentialHistogramMetrics(
		generationOpts.count.ExponentialHistogramMetricsCount,
		generationOpts.count.ExponentialHistogramDataPointCount,
		generationOpts.count.ExponentialHistogramPointAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.ExponentialHistogramBucketCount,
		generationOpts.count.ExponentialHistogramNanValuesCount,
		generationOpts.count.ExponentialHistogramNoRecordCount,
	), sm)

	copyMetricsToScope(GenerateSummaryMetrics(
		generationOpts.count.SummaryMetricsCount,
		generationOpts.count.SummaryDataPointCount,
		generationOpts.count.SummaryPointAttributeCount,
		generationOpts.scopeAttributeCount,
		generationOpts.resourceAttributeCount,
		generationOpts.count.SummaryQuantileCount,
		generationOpts.count.SummaryNanValuesCount,
		generationOpts.count.SummaryNoRecordCount,
	), sm)

	return metrics
}

// Helper function to copy metrics from one scope to another
func copyMetricsToScope(source pmetric.Metrics, targetScope *pmetric.ScopeMetrics) {
	if source.ResourceMetrics().Len() == 0 {
		return
	}

	rm := source.ResourceMetrics().At(0)
	if rm.ScopeMetrics().Len() == 0 {
		return
	}

	sourceMetrics := rm.ScopeMetrics().At(0).Metrics()
	for i := 0; i < sourceMetrics.Len(); i++ {
		metric := sourceMetrics.At(i)
		newMetric := targetScope.Metrics().AppendEmpty()
		metric.CopyTo(newMetric)
	}
}

// GenerateGaugeMetrics generates a set of gauge metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as system.memory.usage0, system.memory.usage1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as gauge.attr0, gauge.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func GenerateGaugeMetrics(numMetrics, totalNumDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	_, sm := setupResourceAndScope(&metrics, numResourceAttributes, numScopeAttributes, "value", "value")

	const baseTimestamp = 1727286182

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.memory.usage" + strconv.Itoa(i))
		m.SetUnit("bytes")
		m.SetDescription("memory usage of the host")

		dpSlice := m.SetEmptyGauge().DataPoints()
		validDataPoints := totalNumDataPoints - numNanValues - numNoRecordedValues

		// Add normal data points
		addDataPoints(dpSlice, i, validDataPoints, numPointAttributes, baseTimestamp, "gauge.attr_", false, false)

		// Add NoRecorded data points
		addDataPoints(dpSlice, i, numNoRecordedValues, numPointAttributes, baseTimestamp, "gauge.attr_", true, false)

		// Add NaN data points
		addDataPoints(dpSlice, i, numNanValues, numPointAttributes, baseTimestamp, "gauge.attr_", false, true)
	}

	return metrics
}

// Helper function to add data points with common attributes
func addDataPoints(dpSlice pmetric.NumberDataPointSlice, metricIndex, count, numPointAttributes int, baseTimestamp int64, attrPrefix string, noRecorded bool, useNaN bool) {
	for j := 0; j < count; j++ {
		dp := dpSlice.AppendEmpty()

		if useNaN {
			dp.SetDoubleValue(math.NaN())
		} else {
			dp.SetIntValue(int64(metricIndex))
		}

		if noRecorded {
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		} else {
			dp.SetFlags(pmetric.DefaultDataPointFlags)
		}

		for k := 0; k < numPointAttributes; k++ {
			dp.Attributes().PutStr(attrPrefix+strconv.Itoa(k), "1")
		}

		setTimestamps(dp, baseTimestamp, int64(j))
	}
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
func GenerateSumMetrics(numMetrics, totalNumDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNoRecordedValue int, numNanValue int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	_, sm := setupResourceAndScope(&metrics, numResourceAttributes, numScopeAttributes, "value", "value")

	const baseTimestamp = 1727286182

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.cpu.time" + strconv.Itoa(i))
		m.SetUnit("s")
		m.SetDescription("cpu time of the host")

		sum := m.SetEmptySum()
		if i%2 == 0 {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		if i%3 == 0 {
			sum.SetIsMonotonic(true)
		} else {
			sum.SetIsMonotonic(false)
		}

		dpSlice := sum.DataPoints()
		validPoints := totalNumDataPoints - numNoRecordedValue - numNanValue

		// Add normal data points
		addDataPoints(dpSlice, i, validPoints, numPointAttributes, baseTimestamp, "sum.attr_", false, false)

		// Add NoRecorded data points
		addDataPoints(dpSlice, i, numNoRecordedValue, numPointAttributes, baseTimestamp, "sum.attr_", true, false)

		// Add Nan data points
		addDataPoints(dpSlice, i, numNanValue, numPointAttributes, baseTimestamp, "sum.attr_", false, true)
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
func GenerateHistogramMetrics(numMetrics, totalNumDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	_, sm := setupResourceAndScope(&metrics, numResourceAttributes, numScopeAttributes, "value", "value")

	const baseTimestamp = 1727286182

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server")

		histogram := m.SetEmptyHistogram()
		if i%2 == 0 {
			histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		dpSlice := histogram.DataPoints()
		validDataPoints := totalNumDataPoints - numNanValues - numNoRecordedValues

		// Define these once
		explicitBounds := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
		bucketCounts := []uint64{1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1}

		// Add normal data points
		addHistogramDataPoints(dpSlice, validDataPoints, numPointAttributes, baseTimestamp, "histogram.attr_", false, explicitBounds, bucketCounts, 0, 12)

		// Add NaN data points
		addHistogramDataPoints(dpSlice, numNanValues, numPointAttributes, baseTimestamp, "histogram.attr_", false, explicitBounds, bucketCounts, math.NaN(), math.NaN())

		// Add NoRecorded data points
		addHistogramDataPoints(dpSlice, numNoRecordedValues, numPointAttributes, baseTimestamp, "histogram.attr_", true, explicitBounds, bucketCounts, 0, 12)
	}

	return metrics
}

// Helper function to add histogram data points
func addHistogramDataPoints(dpSlice pmetric.HistogramDataPointSlice, count, numPointAttributes int, baseTimestamp int64, attrPrefix string, noRecorded bool, explicitBounds []float64, bucketCounts []uint64, min, max float64) {
	for j := 0; j < count; j++ {
		dp := dpSlice.AppendEmpty()
		dp.SetCount(30)
		dp.SetSum(35)

		if noRecorded {
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		} else {
			dp.SetFlags(pmetric.DefaultDataPointFlags)
		}

		for k := 0; k < numPointAttributes; k++ {
			dp.Attributes().PutStr(attrPrefix+strconv.Itoa(k), "1")
		}

		timestamp := pcommon.NewTimestampFromTime(time.Unix(baseTimestamp+int64(j), 0))
		dp.SetStartTimestamp(timestamp)
		dp.SetTimestamp(timestamp)

		dp.ExplicitBounds().FromRaw(explicitBounds)
		dp.BucketCounts().FromRaw(bucketCounts)
		dp.SetMin(min)
		dp.SetMax(max)
	}
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
	numMetrics, totalNumDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes, numBucketCount int, numNanValues int, numNoRecordedValues int,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	_, sm := setupResourceAndScope(&metrics, numResourceAttributes, numScopeAttributes, "value", "value")

	const baseTimestamp = 1727286182
	fixedPattern := []uint64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10}

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server but in exponential histogram format")

		expHistogram := m.SetEmptyExponentialHistogram()
		if i%2 == 0 {
			expHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			expHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}

		dpSlice := expHistogram.DataPoints()
		validDataPoints := totalNumDataPoints - numNanValues - numNoRecordedValues

		// Add normal data points
		addExpHistogramDataPoints(dpSlice, validDataPoints, numPointAttributes, baseTimestamp, "exponential.histogram.attr_", false, fixedPattern, numBucketCount, 1.0)

		// Add NaN data points
		addExpHistogramDataPoints(dpSlice, numNanValues, numPointAttributes, baseTimestamp, "exponential.histogram.attr_", false, fixedPattern, numBucketCount, math.NaN())

		// Add NoRecorded data points
		addExpHistogramDataPoints(dpSlice, numNoRecordedValues, numPointAttributes, baseTimestamp, "exponential.histogram.attr_", true, fixedPattern, numBucketCount, 1.0)
	}

	return metrics
}

// Helper function to add exponential histogram data points
func addExpHistogramDataPoints(dpSlice pmetric.ExponentialHistogramDataPointSlice, count, numPointAttributes int, baseTimestamp int64, attrPrefix string, noRecorded bool, fixedPattern []uint64, numBucketCount int, sumValue float64) {
	for j := 0; j < count; j++ {
		dp := dpSlice.AppendEmpty()
		dp.SetScale(2)

		timestamp := pcommon.NewTimestampFromTime(time.Unix(baseTimestamp+int64(j), 0))
		dp.SetStartTimestamp(timestamp)
		dp.SetTimestamp(timestamp)

		dp.SetSum(sumValue)
		dp.SetMin(0)
		dp.SetMax(1)
		dp.SetZeroCount(0)
		dp.SetCount(1)

		if noRecorded {
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}

		for k := 0; k < numPointAttributes; k++ {
			dp.Attributes().PutStr(attrPrefix+strconv.Itoa(k), "1")
		}

		buckets := make([]uint64, numBucketCount)
		copy(buckets, fixedPattern)

		dp.Negative().SetOffset(1)
		dp.Negative().BucketCounts().FromRaw(buckets)

		dp.Positive().SetOffset(1)
		dp.Positive().BucketCounts().FromRaw(buckets)
	}
}

// GenerateSummaryMetrics generates a set of summary metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as zk.duration0, zk.duration1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as summary.attr0, summary.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func GenerateSummaryMetrics(numMetrics, totalNumDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes, numQuantileCount int, numNanValues int, numNoRecordedValues int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	_, sm := setupResourceAndScope(&metrics, numResourceAttributes, numScopeAttributes, "value", "value")

	const baseTimestamp = 1727286182

	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("zk.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")

		dpSlice := m.SetEmptySummary().DataPoints()
		validDataPoints := totalNumDataPoints - numNanValues - numNoRecordedValues

		// Add normal data points
		addSummaryDataPoints(dpSlice, validDataPoints, numPointAttributes, baseTimestamp, "summary.attr_", false, numQuantileCount, false)

		// Add NaN data points
		addSummaryDataPoints(dpSlice, numNanValues, numPointAttributes, baseTimestamp, "summary.attr_", false, numQuantileCount, true)

		// Add NoRecorded data points
		addSummaryDataPoints(dpSlice, numNoRecordedValues, numPointAttributes, baseTimestamp, "summary.attr_", true, numQuantileCount, false)
	}

	return metrics
}

// Helper function to add summary data points
func addSummaryDataPoints(dpSlice pmetric.SummaryDataPointSlice, count, numPointAttributes int, baseTimestamp int64, attrPrefix string, noRecorded bool, numQuantileCount int, useNaN bool) {
	for j := 0; j < count; j++ {
		dp := dpSlice.AppendEmpty()

		timestamp := pcommon.NewTimestampFromTime(time.Unix(baseTimestamp+int64(j), 0))
		dp.SetStartTimestamp(timestamp)
		dp.SetTimestamp(timestamp)

		dp.SetCount(uint64(j))
		if useNaN {
			dp.SetSum(math.NaN())
		} else {
			dp.SetSum(float64(j))
		}

		if noRecorded {
			dp.SetFlags(dp.Flags().WithNoRecordedValue(true))
		}

		for k := 0; k < numPointAttributes; k++ {
			dp.Attributes().PutStr(attrPrefix+strconv.Itoa(k), "1")
		}

		for q := 0; q < numQuantileCount; q++ {
			qv := dp.QuantileValues().AppendEmpty()
			qv.SetQuantile(float64(q) / float64(numQuantileCount))
			qv.SetValue(float64(j + q))
		}
	}
}
