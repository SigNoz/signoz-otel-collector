package signozmeterconnector

import (
	"bytes"
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	v1 "github.com/SigNoz/signoz-otel-collector/pkg/metering/v1"
	"github.com/jonboulle/clockwork"
)

const (
	metricKeySeparator = string(byte(0))
)

// meterConnector records count and size of spans, metrics data points, log records
// and emits them onto a metrics pipeline.
type meterConnector struct {
	lock            sync.Mutex
	logger          *zap.Logger
	config          Config
	metricsConsumer consumer.Metrics
	logsMeter       metering.Logs
	tracesMeter     metering.Traces
	metricsMeter    metering.Metrics
	dimensions      []ResourceDimensions
	dimensionsData  map[resourceMetricKey]pcommon.Map
	data            map[resourceMetricKey]meterMetrics
	clock           clockwork.Clock
	started         bool
	keyBuf          *bytes.Buffer
	ticker          clockwork.Ticker
	done            chan struct{}
	shutdownOnce    sync.Once
}

type resourceMetricKey string

type meterMetrics struct {
	SpanCount            int
	SpanSize             int
	MetricDataPointCount int
	MetricDataPointSize  int
	LogCount             int
	LogSize              int
}

type ResourceDimensions struct {
	Name  string
	Value *pcommon.Value
}

func newDimensions(cfgDims []Dimension) []ResourceDimensions {
	if len(cfgDims) == 0 {
		return nil
	}
	dims := make([]ResourceDimensions, len(cfgDims))
	for i := range cfgDims {
		dims[i].Name = cfgDims[i].Name
		if cfgDims[i].Default != nil {
			val := pcommon.NewValueStr(*cfgDims[i].Default)
			dims[i].Value = &val
		}
	}
	return dims
}

// initialize the signozmeterconnector
func newConnector(logger *zap.Logger, config component.Config, clock clockwork.Clock) (*meterConnector, error) {
	logger.Info("building signozmeterconnector with config", zap.Any("config", config))
	cfg := config.(*Config)

	return &meterConnector{
		logger:         logger,
		config:         *cfg,
		dimensions:     newDimensions(cfg.Dimensions),
		logsMeter:      v1.NewLogs(logger),
		tracesMeter:    v1.NewTraces(logger),
		metricsMeter:   v1.NewMetrics(logger),
		keyBuf:         bytes.NewBuffer(make([]byte, 0, 1024)),
		dimensionsData: map[resourceMetricKey]pcommon.Map{},
		data:           map[resourceMetricKey]meterMetrics{},
		clock:          clock,
		ticker:         clock.NewTicker(cfg.MetricsFlushInterval),
		done:           make(chan struct{}),
	}, nil
}

func (*meterConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (meterconnector *meterConnector) Start(ctx context.Context, host component.Host) error {
	meterconnector.logger.Info("starting signozmeterconnector")

	meterconnector.started = true
	go func() {
		for {
			select {
			case <-meterconnector.done:
				return
			case <-meterconnector.ticker.Chan():
				meterconnector.exportMetrics(ctx)
			}
		}
	}()

	return nil
}

func (meterconnector *meterConnector) Shutdown(ctx context.Context) error {
	meterconnector.shutdownOnce.Do(func() {
		meterconnector.logger.Info("shutting down signozmeterconnector")

		if meterconnector.started {
			// flush all the inmemory metrics we have before shutting down
			meterconnector.exportMetrics(ctx)
			meterconnector.logger.Info("stopping ticker")
			meterconnector.ticker.Stop()
			meterconnector.done <- struct{}{}
			meterconnector.started = false
		}
	})
	return nil
}

func (meterConnector *meterConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	meterConnector.lock.Lock()
	meterConnector.aggregateMeterMetricsFromTraces(traces)
	meterConnector.lock.Unlock()
	return nil
}

func (meterConnector *meterConnector) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	meterConnector.lock.Lock()
	meterConnector.aggregateMeterMetricsFromMetrics(metrics)
	meterConnector.lock.Unlock()
	return nil
}

func (meterConnector *meterConnector) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	meterConnector.lock.Lock()
	meterConnector.aggregateMeterMetricsFromLogs(logs)
	meterConnector.lock.Unlock()
	return nil
}

func (meterconnector *meterConnector) exportMetrics(ctx context.Context) {
	meterconnector.lock.Lock()

	metrics := meterconnector.buildMetrics()
	meterconnector.resetState()

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	meterconnector.lock.Unlock()

	if metrics.DataPointCount() == 0 {
		return
	}
	if err := meterconnector.metricsConsumer.ConsumeMetrics(ctx, metrics); err != nil {
		meterconnector.logger.Error("failed ConsumeMetrics", zap.Error(err))
		return
	}
}

func (meterconnector *meterConnector) buildMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	timestamp := pcommon.NewTimestampFromTime(meterconnector.clock.Now())

	for resourceMetricKey, meterMetrics := range meterconnector.data {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName("signozmeterconnector")
		// for each resource metric key we need 6 metrics (2 for each telemetry data type)
		scopeMetrics.Metrics().EnsureCapacity(6 * len(meterconnector.data))

		// generate the metrics from the aggregated telemetry data collected in memory
		meterconnector.collectLogMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)
		meterconnector.collectTraceMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)
		meterconnector.collectMetricMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)
	}

	return metrics
}

// generate metrics from the stored dimensions and data for logs
func (meterconnector *meterConnector) collectLogMeterMetrics(scopeMetrics pmetric.ScopeMetrics, resourceMetricKey resourceMetricKey, meterMetrics meterMetrics, timestamp pcommon.Timestamp) {
	if meterMetrics.LogCount == 0 {
		return
	}

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameLogsCount)
	metric.SetDescription(metricDescLogsCount)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint := metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.LogCount))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameLogsSize)
	metric.SetDescription(metricDescLogsSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.LogSize))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}
}

// generate metrics from the stored dimensions and data for spans
func (meterconnector *meterConnector) collectTraceMeterMetrics(scopeMetrics pmetric.ScopeMetrics, resourceMetricKey resourceMetricKey, meterMetrics meterMetrics, timestamp pcommon.Timestamp) {
	if meterMetrics.SpanCount == 0 {
		return
	}

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameSpansCount)
	metric.SetDescription(metricDescSpansCount)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint := metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.SpanCount))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameSpansSize)
	metric.SetDescription(metricDescSpansSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.SpanSize))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}
}

// generate metrics from the stored dimensions and data for metrics
func (meterconnector *meterConnector) collectMetricMeterMetrics(scopeMetrics pmetric.ScopeMetrics, resourceMetricKey resourceMetricKey, meterMetrics meterMetrics, timestamp pcommon.Timestamp) {
	if meterMetrics.MetricDataPointCount == 0 {
		return
	}

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameMetricsDataPointsCount)
	metric.SetDescription(metricDescMetricsDataPointsCount)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint := metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.MetricDataPointCount))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameMetricsDataPointsSize)
	metric.SetDescription(metricDescMetricsDataPointsSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.MetricDataPointSize))
	// Set dimensions as labels
	if dimensions, ok := meterconnector.dimensionsData[resourceMetricKey]; ok {
		dimensions.CopyTo(dataPoint.Attributes())
	}
}

// reset the in-memory stored dimensions/metrics state
func (meterconnector *meterConnector) resetState() {
	// reset the state here as we are using delta temporality
	meterconnector.data = map[resourceMetricKey]meterMetrics{}
	meterconnector.dimensionsData = map[resourceMetricKey]pcommon.Map{}
}

// generate the aggregated data from the traces data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromTraces(traces ptrace.Traces) {
	// generate raw data from inmemeory storage from traces
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpans := traces.ResourceSpans().At(i)
		resourceAttributes := resourceSpans.Resource().Attributes()
		resourceMetricKey := meterconnector.buildKeyFromResourceBasedOnDimensions(resourceAttributes)
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttributes)
		// if the given dimensions are not present on the dimensions map add them
		if _, ok := meterconnector.dimensionsData[resourceMetricKey]; !ok {
			meterconnector.dimensionsData[resourceMetricKey] = resourceMetricDimensions
		}

		count := meterconnector.tracesMeter.CountPerResource(resourceSpans)
		size := meterconnector.tracesMeter.SizePerResource(resourceSpans)
		// update the meter metrics data against the resource metric key
		if meterMetricData, ok := meterconnector.data[resourceMetricKey]; !ok {
			meterconnector.data[resourceMetricKey] = meterMetrics{
				SpanCount: count,
				SpanSize:  size,
			}
		} else {
			meterMetricData.SpanSize += size
			meterMetricData.SpanCount += count
			meterconnector.data[resourceMetricKey] = meterMetricData
		}
	}
}

// generate the aggregated data from the metrics data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromMetrics(metrics pmetric.Metrics) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		resourceMetric := metrics.ResourceMetrics().At(i)
		resourceAttr := resourceMetric.Resource().Attributes()
		resourceMetricKey := meterconnector.buildKeyFromResourceBasedOnDimensions(resourceAttr)
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttr)

		// if the given dimensions are not present on the dimensions map add them
		if _, ok := meterconnector.dimensionsData[resourceMetricKey]; !ok {
			meterconnector.dimensionsData[resourceMetricKey] = resourceMetricDimensions
		}

		count := meterconnector.metricsMeter.CountPerResource(resourceMetric)
		size := meterconnector.metricsMeter.SizePerResource(resourceMetric)
		// update the meter metrics data against the resource metric key
		if meterMetricData, ok := meterconnector.data[resourceMetricKey]; !ok {
			meterconnector.data[resourceMetricKey] = meterMetrics{
				MetricDataPointCount: count,
				MetricDataPointSize:  size,
			}
		} else {
			meterMetricData.MetricDataPointSize += size
			meterMetricData.MetricDataPointCount += count
			meterconnector.data[resourceMetricKey] = meterMetricData
		}

	}
}

// generate the aggregated data from the logs data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromLogs(logs plog.Logs) {
	// generate raw data for inmemory storage from logs
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		resourceAttr := resourceLogs.Resource().Attributes()
		resourceMetricKey := meterconnector.buildKeyFromResourceBasedOnDimensions(resourceAttr)
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttr)

		// if the given dimensions are not present on the dimensions map add them
		if _, ok := meterconnector.dimensionsData[resourceMetricKey]; !ok {
			meterconnector.dimensionsData[resourceMetricKey] = resourceMetricDimensions
		}

		count := meterconnector.logsMeter.CountPerResource(resourceLogs)
		size := meterconnector.logsMeter.SizePerResource(resourceLogs)
		// update the meter metrics data against the resource metric key
		if meterMetricData, ok := meterconnector.data[resourceMetricKey]; !ok {
			meterconnector.data[resourceMetricKey] = meterMetrics{
				LogCount: count,
				LogSize:  size,
			}
		} else {
			meterMetricData.LogSize += size
			meterMetricData.LogCount += count
			meterconnector.data[resourceMetricKey] = meterMetricData
		}

	}
}

// buildKeyFromResourceBasedOnDimensions builds the resourceMetricKey based on configured dimensions and passed resourceAttributes
func (meterconnector *meterConnector) buildKeyFromResourceBasedOnDimensions(resourceAttributes pcommon.Map) resourceMetricKey {
	// reset the buffer before use to flush any unwanted / previous items in the buffer
	meterconnector.keyBuf.Reset()

	for _, dimension := range meterconnector.dimensions {
		dimensionValue, _ := meterconnector.getDimensionValue(dimension, resourceAttributes)
		meterconnector.concatDimensionValue(meterconnector.keyBuf, dimensionValue, true)
	}

	return resourceMetricKey(meterconnector.keyBuf.String())
}

// buildDimensionsMapFromResourceAttributes builds the map to be stored against the resourceMetricKey
func (meterconnector *meterConnector) buildDimensionsMapFromResourceAttributes(resourceAttributes pcommon.Map) pcommon.Map {
	dimensionsMap := pcommon.NewMap()
	dimensionsMap.EnsureCapacity(len(meterconnector.dimensions))

	for _, dimension := range meterconnector.dimensions {
		// get the dimension value from the resource attributes
		dimensionValue, ok := meterconnector.getDimensionValue(dimension, resourceAttributes)
		if ok {
			dimensionsMap.PutStr(dimension.Name, dimensionValue)
		}

	}

	return dimensionsMap
}

// getDimensionValue iterates over the attributes and find the dimension value on order basis.
// if nothing is found it returns the default value (if specified) else an empty string
func (meterconnector *meterConnector) getDimensionValue(dimension ResourceDimensions, attributes ...pcommon.Map) (string, bool) {
	for _, attrs := range attributes {
		if attr, exists := attrs.Get(dimension.Name); exists {
			return attr.AsString(), true
		}
	}

	// Set the default if configured, otherwise this metric will have no Value set for the Dimension.
	if dimension.Value != nil {
		return dimension.Value.AsString(), true
	}

	return "", false
}

func (meterconnector *meterConnector) concatDimensionValue(dest *bytes.Buffer, value string, prefixSep bool) {
	if prefixSep {
		dest.WriteString(metricKeySeparator)
	}
	dest.WriteString(value)
}
