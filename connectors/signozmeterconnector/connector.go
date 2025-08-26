package signozmeterconnector

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	v1 "github.com/SigNoz/signoz-otel-collector/pkg/metering/v1"
	"github.com/google/uuid"
)

// meterConnector records count and size of spans, metrics data points, log records
// and emits them onto a metrics pipeline.
type meterConnector struct {
	logger                 *zap.Logger
	id                     uuid.UUID
	config                 Config
	metricsConsumer        consumer.Metrics
	logsMeter              metering.Logs
	tracesMeter            metering.Traces
	metricsMeter           metering.Metrics
	dimensions             map[string]struct{}
	aggregatedMeterMetrics *aggregatedMeterMetrics
	started                bool
	ticker                 *time.Ticker
	done                   chan struct{}
	wg                     sync.WaitGroup
	telemetry              *meterTelemetry
}

func newDimensionsMap(cfgDims []Dimension) map[string]struct{} {
	dimensionsMap := map[string]struct{}{}
	for _, key := range cfgDims {
		dimensionsMap[key.Name] = struct{}{}
	}

	return dimensionsMap
}

// initialize the signozmeterconnector
func newConnector(logger *zap.Logger, settings connector.Settings, config component.Config) (*meterConnector, error) {
	cfg := config.(*Config)

	meterTelemetry, err := newMeterTelemetry(settings)
	if err != nil {
		return nil, err
	}

	return &meterConnector{
		logger:                 logger,
		id:                     uuid.New(),
		config:                 *cfg,
		dimensions:             newDimensionsMap(cfg.Dimensions),
		logsMeter:              v1.NewLogs(logger),
		tracesMeter:            v1.NewTraces(logger),
		metricsMeter:           v1.NewMetrics(logger),
		aggregatedMeterMetrics: newAggregatedMeterMetrics(),
		ticker:                 time.NewTicker(cfg.MetricsFlushInterval),
		done:                   make(chan struct{}),
		telemetry:              meterTelemetry,
	}, nil
}

func (*meterConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (meterconnector *meterConnector) Start(ctx context.Context, host component.Host) error {
	meterconnector.started = true
	meterconnector.wg.Add(1)
	go func() {
		defer meterconnector.wg.Done()
		for {
			select {
			case <-meterconnector.done:
				return
			case <-meterconnector.ticker.C:
				_ = meterconnector.exportMetrics(ctx)
			}
		}
	}()

	return nil
}

func (meterconnector *meterConnector) Shutdown(ctx context.Context) error {
	if meterconnector.started {
		// flush all the inmemory metrics we have before shutting down
		_ = meterconnector.exportMetrics(ctx)
		meterconnector.ticker.Stop()
		meterconnector.done <- struct{}{}
		meterconnector.started = false
		meterconnector.wg.Wait()
	}

	return nil
}

func (meterConnector *meterConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	meterConnector.aggregateMeterMetricsFromTraces(traces)
	meterConnector.telemetry.record(ctx, connectorRoleReceiver, int64(traces.SpanCount()),
		attribute.KeyValue{Key: attribute.Key(acceptedAttribute), Value: attribute.BoolValue(true)},
		attribute.KeyValue{Key: attribute.Key(signalAttribute), Value: attribute.StringValue("traces")},
	)
	return nil
}

func (meterConnector *meterConnector) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	meterConnector.aggregateMeterMetricsFromMetrics(metrics)
	meterConnector.telemetry.record(ctx, connectorRoleReceiver, int64(metrics.DataPointCount()),
		attribute.KeyValue{Key: attribute.Key(acceptedAttribute), Value: attribute.BoolValue(true)},
		attribute.KeyValue{Key: attribute.Key(signalAttribute), Value: attribute.StringValue("metrics")},
	)
	return nil
}

func (meterConnector *meterConnector) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	meterConnector.aggregateMeterMetricsFromLogs(logs)
	meterConnector.telemetry.record(ctx, connectorRoleReceiver, int64(logs.LogRecordCount()),
		attribute.KeyValue{Key: attribute.Key(acceptedAttribute), Value: attribute.BoolValue(true)},
		attribute.KeyValue{Key: attribute.Key(signalAttribute), Value: attribute.StringValue("logs")},
	)
	return nil
}

func (meterconnector *meterConnector) exportMetrics(ctx context.Context) error {
	meterconnector.aggregatedMeterMetrics.Lock()

	metrics := meterconnector.buildMetrics()
	meterconnector.resetState()

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	meterconnector.aggregatedMeterMetrics.Unlock()

	if metrics.DataPointCount() == 0 {
		return nil
	}
	if err := meterconnector.metricsConsumer.ConsumeMetrics(ctx, metrics); err != nil {
		meterconnector.logger.Error("failed ConsumeMetrics", zap.Error(err))
		meterconnector.telemetry.record(ctx, connectorRoleExporter, int64(metrics.DataPointCount()),
			attribute.KeyValue{Key: attribute.Key(sentAttribute), Value: attribute.BoolValue(false)},
			attribute.KeyValue{Key: attribute.Key(signalAttribute), Value: attribute.StringValue("metrics")},
		)
		return err
	}

	meterconnector.telemetry.record(ctx, connectorRoleExporter, int64(metrics.DataPointCount()),
		attribute.KeyValue{Key: attribute.Key(sentAttribute), Value: attribute.BoolValue(true)},
		attribute.KeyValue{Key: attribute.Key(signalAttribute), Value: attribute.StringValue("metrics")},
	)
	return nil
}

func (meterconnector *meterConnector) buildMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	for _, meterMetrics := range meterconnector.aggregatedMeterMetrics.meterMetrics {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName("signozmeterconnector")
		// add connector id for single-writer in case of multiple connectors
		scopeMetrics.Scope().Attributes().PutStr("connector_id", meterconnector.id.String())
		// for each resource metric key we need 6 metrics (2 for each telemetry data type)
		scopeMetrics.Metrics().EnsureCapacity(6 * len(meterconnector.aggregatedMeterMetrics.meterMetrics))

		// generate the metrics from the aggregated telemetry data collected in memory
		meterconnector.collectLogMeterMetrics(scopeMetrics, meterMetrics, timestamp)
		meterconnector.collectTraceMeterMetrics(scopeMetrics, meterMetrics, timestamp)
		meterconnector.collectMetricMeterMetrics(scopeMetrics, meterMetrics, timestamp)
	}

	return metrics
}

// generate metrics from the stored dimensions and data for logs
func (meterconnector *meterConnector) collectLogMeterMetrics(scopeMetrics pmetric.ScopeMetrics, meterMetrics *meterMetric, timestamp pcommon.Timestamp) {
	if meterMetrics.logCount == 0 {
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
	dataPoint.SetIntValue(int64(meterMetrics.logCount))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameLogsSize)
	metric.SetDescription(metricDescLogsSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.logSize))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}
}

// generate metrics from the stored dimensions and data for spans
func (meterconnector *meterConnector) collectTraceMeterMetrics(scopeMetrics pmetric.ScopeMetrics, meterMetrics *meterMetric, timestamp pcommon.Timestamp) {
	if meterMetrics.spanCount == 0 {
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
	dataPoint.SetIntValue(int64(meterMetrics.spanCount))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameSpansSize)
	metric.SetDescription(metricDescSpansSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.spanSize))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}
}

// generate metrics from the stored dimensions and data for metrics
func (meterconnector *meterConnector) collectMetricMeterMetrics(scopeMetrics pmetric.ScopeMetrics, meterMetrics *meterMetric, timestamp pcommon.Timestamp) {
	if meterMetrics.metricDataPointCount == 0 {
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
	dataPoint.SetIntValue(int64(meterMetrics.metricDataPointCount))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}

	metric = scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricNameMetricsDataPointsSize)
	metric.SetDescription(metricDescMetricsDataPointsSize)
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	metric.Sum().SetIsMonotonic(true)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(timestamp)
	dataPoint.SetIntValue(int64(meterMetrics.metricDataPointSize))
	if meterMetrics.attrs.Len() != 0 {
		meterMetrics.attrs.CopyTo(dataPoint.Attributes())
	}
}

// reset the in-memory stored dimensions/metrics state
func (meterconnector *meterConnector) resetState() {
	// reset the state here as we are using delta temporality
	meterconnector.aggregatedMeterMetrics.Purge()
}

// generate the aggregated data from the traces data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromTraces(traces ptrace.Traces) {
	// generate raw data from inmemeory storage from traces
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpans := traces.ResourceSpans().At(i)
		resourceAttributes := resourceSpans.Resource().Attributes()
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttributes)

		count := meterconnector.tracesMeter.CountPerResource(resourceSpans)
		size := meterconnector.tracesMeter.SizePerResource(resourceSpans)
		meterconnector.aggregatedMeterMetrics.UpdateSpanMeterMetrics(resourceMetricDimensions, count, size)
	}
}

// generate the aggregated data from the metrics data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromMetrics(metrics pmetric.Metrics) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		resourceMetric := metrics.ResourceMetrics().At(i)
		resourceAttr := resourceMetric.Resource().Attributes()
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttr)

		count := meterconnector.metricsMeter.CountPerResource(resourceMetric)
		size := meterconnector.metricsMeter.SizePerResource(resourceMetric)
		// update the meter metrics data against the resource metric key
		meterconnector.aggregatedMeterMetrics.UpdateMetricDataPointsMeterMetrics(resourceMetricDimensions, count, size)
	}
}

// generate the aggregated data from the logs data stream
func (meterconnector *meterConnector) aggregateMeterMetricsFromLogs(logs plog.Logs) {
	// generate raw data for inmemory storage from logs
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		resourceAttr := resourceLogs.Resource().Attributes()
		resourceMetricDimensions := meterconnector.buildDimensionsMapFromResourceAttributes(resourceAttr)

		count := meterconnector.logsMeter.CountPerResource(resourceLogs)
		size := meterconnector.logsMeter.SizePerResource(resourceLogs)
		// update the meter metrics data against the resource metric key
		meterconnector.aggregatedMeterMetrics.UpdateLogMeterMetrics(resourceMetricDimensions, count, size)
	}
}

// buildDimensionsMapFromResourceAttributes builds the map to be stored against the resourceMetricKey
func (meterconnector *meterConnector) buildDimensionsMapFromResourceAttributes(resourceAttributes pcommon.Map) pcommon.Map {
	dimensionsMap := pcommon.NewMap()

	resourceAttributes.Range(func(k string, v pcommon.Value) bool {
		if _, ok := meterconnector.dimensions[k]; ok {
			dimensionsMap.PutStr(k, v.AsString())
		}
		return true
	})

	return dimensionsMap
}
