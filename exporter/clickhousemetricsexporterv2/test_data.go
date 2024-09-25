package clickhousemetricsexporterv2

import (
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// generateGaugeMetrics generates a set of gauge metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as system.memory.usage0, system.memory.usage1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as gauge.attr0, gauge.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func generateGaugeMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
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
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		for j := 0; j < numDataPoints; j++ {
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("gauge.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182, 0)))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1727286182, 0)))
		}
	}
	return metrics
}

// generateSumMetrics generates a set of sum metrics with
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
func generateSumMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
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
	timestamp := time.Unix(1727286182, 0)
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.cpu.time" + strconv.Itoa(i))
		m.SetUnit("s")
		m.SetDescription("cpu time of the host")
		dp := m.SetEmptySum().DataPoints().AppendEmpty()
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
			dp.SetIntValue(int64(i))
			dp.SetFlags(pmetric.DefaultDataPointFlags)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("sum.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		}
	}
	return metrics
}

// generateHistogramMetrics generates a set of histogram metrics with
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
func generateHistogramMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
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
	timestamp := time.Unix(1727286182, 0)
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server")
		dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		if i%2 == 0 {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp.SetCount(30)
			dp.SetSum(35)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("histogram.attr_"+strconv.Itoa(k), "1")
			}
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.ExplicitBounds().FromRaw([]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
			dp.BucketCounts().FromRaw([]uint64{1, 1, 1, 1, 1, 5, 1, 1, 1, 1, 1, 1, 12, 1, 1, 1, 1, 1, 1, 1})
			dp.SetMin(0)
			dp.SetMax(12)
		}
	}
	return metrics
}

// generateExponentialHistogramMetrics generates a set of exponential histogram metrics with
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
func generateExponentialHistogramMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
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
	timestamp := time.Unix(1727286182, 0)
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.server.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("server duration of the http server but in exponential histogram format")
		dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		if i%2 == 0 {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		} else {
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
		for j := 0; j < numDataPoints; j++ {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
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

// generateSummaryMetrics generates a set of summary metrics with
// the given number of metrics, data points, point attributes,
// scope attributes, and resource attributes.
// the metrics will be named as zk.duration0, zk.duration1, etc.
// the data points will have the value as the index of the data point
// the point attributes will be named as summary.attr0, summary.attr1, etc.
// the scope attributes will be named as scope.attr0, scope.attr1, etc.
// the resource attributes will be named as resource.attr0, resource.attr1, etc.
func generateSummaryMetrics(numMetrics, numDataPoints, numPointAttributes, numScopeAttributes, numResourceAttributes int) pmetric.Metrics {
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
	timestamp := time.Unix(1727286182, 0)
	for i := 0; i < numMetrics; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetName("zk.duration" + strconv.Itoa(i))
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		dp := m.SetEmptySummary().DataPoints().AppendEmpty()
		for j := 0; j < numDataPoints; j++ {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetCount(1)
			dp.SetSum(1)
			for k := 0; k < numPointAttributes; k++ {
				dp.Attributes().PutStr("summary.attr_"+strconv.Itoa(k), "1")
			}
			quantileValues := dp.QuantileValues().AppendEmpty()
			quantileValues.SetValue(1)
			quantileValues.SetQuantile(1)
		}
	}
	return metrics
}
