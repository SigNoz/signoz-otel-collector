// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signozspanmetricsprocessor

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/SigNoz/signoz-otel-collector/processor/signozspanmetricsprocessor/internal/cache"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	serviceNameKey             = conventions.AttributeServiceName
	operationKey               = "operation" // is there a constant we can refer to?
	spanKindKey                = "span.kind"
	statusCodeKey              = "status.code"
	TagHTTPStatusCode          = conventions.AttributeHTTPStatusCode
	metricKeySeparator         = string(byte(0))
	traceIDKey                 = "trace_id"
	defaultDimensionsCacheSize = 1000
	resourcePrefix             = "resource_"
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = durationToMillis(maxDuration)

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

type metricKey string

type processorImp struct {
	lock   sync.RWMutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.Traces

	// Additional dimensions to add to metrics.
	callDimensions         []Dimension
	dimensions             []Dimension
	dbCallDimensions       []Dimension
	externalCallDimensions []Dimension
	// The starting time of the data points.
	startTime time.Time

	// Call & Error counts.
	callSum map[metricKey]int64

	// Latency histogram.
	latencyCount        map[metricKey]uint64
	latencySum          map[metricKey]float64
	latencyBucketCounts map[metricKey][]uint64

	latencyBounds []float64

	dbLatencyCount map[metricKey]uint64
	dbLatencySum   map[metricKey]float64

	externalCallLatencyCount map[metricKey]uint64
	externalCallLatencySum   map[metricKey]float64

	// A cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions             *cache.Cache
	callMetricKeyToDimensions         *cache.Cache
	dbMetricKeyToDimensions           *cache.Cache
	externalCallMetricKeyToDimensions *cache.Cache
}

func newProcessor(logger *zap.Logger, config config.Processor, nextConsumer consumer.Traces) (*processorImp, error) {
	logger.Info("Building signozspanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	if err := validateDimensions(pConfig.Dimensions); err != nil {
		return nil, err
	}

	callDimensions := []Dimension{
		// {Name: operationKey},
		// {Name: spanKindKey},
		// {Name: statusCodeKey},
		{Name: TagHTTPStatusCode},
	}
	callDimensions = append(callDimensions, pConfig.Dimensions...)

	dbCallDimensions := []Dimension{
		{Name: "db.system"},
		{Name: "db.name"},
	}
	dbCallDimensions = append(dbCallDimensions, pConfig.Dimensions...)

	var externalCallDimensions []Dimension
	externalCallDimensions = append(externalCallDimensions, pConfig.Dimensions...)

	if pConfig.DimensionsCacheSize <= 0 {
		return nil, fmt.Errorf(
			"invalid cache size: %v, the maximum number of the items in the cache should be positive",
			pConfig.DimensionsCacheSize,
		)
	}
	metricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	callMetricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	dbMetricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	externalCallMetricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	return &processorImp{
		logger:                            logger,
		config:                            *pConfig,
		startTime:                         time.Now(),
		callSum:                           make(map[metricKey]int64),
		latencyBounds:                     bounds,
		latencySum:                        make(map[metricKey]float64),
		latencyCount:                      make(map[metricKey]uint64),
		latencyBucketCounts:               make(map[metricKey][]uint64),
		dbLatencySum:                      make(map[metricKey]float64),
		dbLatencyCount:                    make(map[metricKey]uint64),
		externalCallLatencySum:            make(map[metricKey]float64),
		externalCallLatencyCount:          make(map[metricKey]uint64),
		nextConsumer:                      nextConsumer,
		dimensions:                        pConfig.Dimensions,
		callDimensions:                    callDimensions,
		dbCallDimensions:                  dbCallDimensions,
		externalCallDimensions:            externalCallDimensions,
		callMetricKeyToDimensions:         callMetricKeyToDimensionsCache,
		metricKeyToDimensions:             metricKeyToDimensionsCache,
		dbMetricKeyToDimensions:           dbMetricKeyToDimensionsCache,
		externalCallMetricKeyToDimensions: externalCallMetricKeyToDimensionsCache,
	}, nil
}

// durationToMillis converts the given duration to the number of milliseconds it represents.
// Note that this can return sub-millisecond (i.e. < 1ms) values as well.
func durationToMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond.Nanoseconds())
}

func mapDurationsToMillis(vs []time.Duration) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = durationToMillis(v)
	}
	return vsm
}

// validateDimensions checks duplicates for reserved dimensions and additional dimensions. Considering
// the usage of Prometheus related exporters, we also validate the dimensions after sanitization.
func validateDimensions(dimensions []Dimension) error {
	labelNames := make(map[string]struct{})
	for _, key := range []string{serviceNameKey, spanKindKey, statusCodeKey} {
		labelNames[key] = struct{}{}
		labelNames[sanitize(key)] = struct{}{}
	}
	labelNames[operationKey] = struct{}{}

	for _, key := range dimensions {
		if _, ok := labelNames[key.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", key.Name)
		}
		labelNames[key.Name] = struct{}{}

		sanitizedName := sanitize(key.Name)
		if sanitizedName == key.Name {
			continue
		}
		if _, ok := labelNames[sanitizedName]; ok {
			return fmt.Errorf("duplicate dimension name %s after sanitization", sanitizedName)
		}
		labelNames[sanitizedName] = struct{}{}
	}

	return nil
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting signozspanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[config.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.String())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.String())

		p.logger.Debug("Looking for signozspanmetrics exporter from available exporters",
			zap.String("signozspanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.String() == p.config.MetricsExporter {
			p.metricsExporter = metricsExp
			p.logger.Info("Found exporter", zap.String("signozspanmetrics-exporter", p.config.MetricsExporter))
			break
		}
	}
	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}
	p.logger.Info("Started signozspanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down signozspanmetricsprocessor")
	return nil
}

// Capabilities implements the consumer interface.
func (p *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	// p.logger.Info("Consuming trace data")

	p.aggregateMetrics(traces)

	m, err := p.buildMetrics()
	if err != nil {
		return err
	}
	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	// Forward trace data unmodified.
	return p.nextConsumer.ConsumeTraces(ctx, traces)
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() (*pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("signozspanmetricsprocessor")

	// Obtain write lock to reset data
	p.lock.Lock()

	if err := p.collectCallMetrics(ilm); err != nil {
		p.lock.Unlock()
		return nil, err
	}
	if err := p.collectLatencyMetrics(ilm); err != nil {
		p.lock.Unlock()
		return nil, err
	}
	if err := p.collectExternalCallMetrics(ilm); err != nil {
		p.lock.Unlock()
		return nil, err
	}
	if err := p.collectDBCallMetrics(ilm); err != nil {
		p.lock.Unlock()
		return nil, err
	}
	p.metricKeyToDimensions.RemoveEvictedItems()
	p.callMetricKeyToDimensions.RemoveEvictedItems()
	p.dbMetricKeyToDimensions.RemoveEvictedItems()
	p.externalCallMetricKeyToDimensions.RemoveEvictedItems()

	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
		p.resetAccumulatedMetrics()
	}

	p.lock.Unlock()

	return &m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.latencyCount {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pmetric.MetricDataTypeHistogram)
		mLatency.SetName("signoz_latency")
		mLatency.Histogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpLatency.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(p.latencyBounds))
		dpLatency.SetBucketCounts(pcommon.NewImmutableUInt64Slice(p.latencyBucketCounts[key]))
		dpLatency.SetCount(p.latencyCount[key])
		dpLatency.SetSum(p.latencySum[key])

		dimensions, err := p.getDimensionsByMetricKey(p.metricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectDBCallMetrics collects the raw latency sum and count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectDBCallMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.dbLatencyCount {
		dbCallSum := ilm.Metrics().AppendEmpty()
		dbCallSum.SetDataType(pmetric.MetricDataTypeSum)
		dbCallSum.SetName("signoz_db_latency_sum")
		dbCallSum.Sum().SetIsMonotonic(true)
		dbCallSum.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dbCallCount := ilm.Metrics().AppendEmpty()
		dbCallCount.SetDataType(pmetric.MetricDataTypeSum)
		dbCallCount.SetName("signoz_db_latency_count")
		dbCallCount.Sum().SetIsMonotonic(true)
		dbCallCount.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCallSumCalls := dbCallSum.Sum().DataPoints().AppendEmpty()
		dpCallSumCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCallSumCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCallSumCalls.SetDoubleVal(p.dbLatencySum[key])

		dpCallCountCalls := dbCallCount.Sum().DataPoints().AppendEmpty()
		dpCallCountCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCallCountCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCallCountCalls.SetIntVal(int64(p.dbLatencyCount[key]))

		dimensions, err := p.getDimensionsByMetricKey(p.dbMetricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpCallSumCalls.Attributes())
		dimensions.CopyTo(dpCallCountCalls.Attributes())
	}
	return nil
}

// collectExternalCallMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectExternalCallMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.externalCallLatencyCount {
		externalCallSum := ilm.Metrics().AppendEmpty()
		externalCallSum.SetDataType(pmetric.MetricDataTypeSum)
		externalCallSum.SetName("signoz_external_call_latency_sum")
		externalCallSum.Sum().SetIsMonotonic(true)
		externalCallSum.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		externalCallCount := ilm.Metrics().AppendEmpty()
		externalCallCount.SetDataType(pmetric.MetricDataTypeSum)
		externalCallCount.SetName("signoz_external_call_latency_count")
		externalCallCount.Sum().SetIsMonotonic(true)
		externalCallCount.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCallSumCalls := externalCallSum.Sum().DataPoints().AppendEmpty()
		dpCallSumCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCallSumCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCallSumCalls.SetDoubleVal(p.externalCallLatencySum[key])

		dpCallCountCalls := externalCallCount.Sum().DataPoints().AppendEmpty()
		dpCallCountCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCallCountCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCallCountCalls.SetIntVal(int64(p.externalCallLatencyCount[key]))

		dimensions, err := p.getDimensionsByMetricKey(p.externalCallMetricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpCallSumCalls.Attributes())
		dimensions.CopyTo(dpCallCountCalls.Attributes())
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.callSum {
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetDataType(pmetric.MetricDataTypeSum)
		mCalls.SetName("signoz_calls_total")
		mCalls.Sum().SetIsMonotonic(true)
		mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCalls := mCalls.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntVal(p.callSum[key])

		dimensions, err := p.getDimensionsByMetricKey(p.callMetricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}
	return nil
}

// getDimensionsByMetricKey gets dimensions from `metricKeyToDimensions` cache.
func (p *processorImp) getDimensionsByMetricKey(cache *cache.Cache, k metricKey) (*pcommon.Map, error) {
	if item, ok := cache.Get(k); ok {
		if attributeMap, ok := item.(pcommon.Map); ok {
			return &attributeMap, nil
		}
		return nil, fmt.Errorf("type assertion of metricKeyToDimensions attributes failed, the key is %q", k)
	}

	return nil, fmt.Errorf("value not found in metricKeyToDimensions cache by key %q", k)
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		r := rspans.Resource()

		attr, ok := r.Attributes().Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := attr.StringVal()
		p.aggregateMetricsForServiceSpans(rspans, serviceName)
	}
}

func (p *processorImp) aggregateMetricsForServiceSpans(rspans ptrace.ResourceSpans, serviceName string) {
	ilsSlice := rspans.ScopeSpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			p.aggregateMetricsForSpan(serviceName, span, rspans.Resource().Attributes())
		}
	}
}

func getRemoteAddress(span ptrace.Span) (string, bool) {
	var addr string

	getPeerAddress := func(attrs pcommon.Map) (string, bool) {
		var addr string
		// Since net.peer.name is readable, it is preferred over net.peer.ip.
		peerName, ok := attrs.Get(conventions.AttributeNetPeerName)
		if ok {
			addr = peerName.StringVal()
			port, ok := attrs.Get(conventions.AttributeNetPeerPort)
			if ok {
				addr += ":" + port.StringVal()
			}
			return addr, true
		}
		peerIp, ok := attrs.Get(conventions.AttributeNetPeerIP)
		if ok {
			addr = peerIp.StringVal()
			port, ok := attrs.Get(conventions.AttributeNetPeerPort)
			if ok {
				addr += ":" + port.StringVal()
			}
			return addr, true
		}
		return "", false
	}

	attrs := span.Attributes()
	_, isRPC := attrs.Get(conventions.AttributeRPCSystem)
	// If the span is an RPC, the remote address is service/method.
	if isRPC {
		service, svcOK := attrs.Get(conventions.AttributeRPCService)
		if svcOK {
			addr = service.StringVal()
		}
		method, methodOK := attrs.Get(conventions.AttributeRPCMethod)
		if methodOK {
			addr += "/" + method.StringVal()
		}
		if addr != "" {
			return addr, true
		}
		// Ideally shouldn't reach here but if for some reason
		// service/method not set for RPC, fallback to peer address.
		return getPeerAddress(attrs)
	}

	// If HTTP host is set, use it.
	host, ok := attrs.Get(conventions.AttributeHTTPHost)
	if ok {
		return host.StringVal(), true
	}

	peerAddress, ok := getPeerAddress(attrs)
	if ok {
		// If the peer address is set and the transport is not unix domain socket, or pipe
		transport, ok := attrs.Get(conventions.AttributeNetTransport)
		if ok && transport.StringVal() == "unix" && transport.StringVal() == "pipe" {
			return "", false
		}
		return peerAddress, true
	}

	// If none of the above is set, check for full URL.
	httpURL, ok := attrs.Get(conventions.AttributeHTTPURL)
	if ok {
		urlValue := httpURL.StringVal()
		// url pattern from godoc [scheme:][//[userinfo@]host][/]path[?query][#fragment]
		if !strings.HasPrefix(urlValue, "http://") && !strings.HasPrefix(urlValue, "https://") {
			urlValue = "http://" + urlValue
		}
		parsedURL, err := url.Parse(urlValue)
		if err != nil {
			return "", false
		}
		return parsedURL.Host, true
	}

	peerService, ok := attrs.Get(conventions.AttributePeerService)
	if ok {
		return peerService.StringVal(), true
	}

	return "", false
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span ptrace.Span, resourceAttr pcommon.Map) {

	// Ideally shouldn't happen but if for some reason span end time is before start time,
	// ignore the span. We don't want to count negative latency.
	if span.EndTimestamp() < span.StartTimestamp() {
		return
	}

	latencyInMilliseconds := float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())

	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latencyInMilliseconds)

	p.lock.Lock()

	callKey := buildKey(serviceName, span, p.callDimensions, resourceAttr)
	p.callCache(serviceName, span, callKey, resourceAttr)
	p.updateCallMetrics(callKey)

	key := buildKey(serviceName, span, p.dimensions, resourceAttr)
	p.cache(serviceName, span, key, resourceAttr)
	p.updateLatencyMetrics(key, latencyInMilliseconds, index)

	spanAttr := span.Attributes()
	remoteAddr, externalCallPresent := getRemoteAddress(span)

	if span.Kind() == ptrace.SpanKindClient && externalCallPresent {
		extraVals := []string{remoteAddr}
		externalCallKey := buildCustomKey(serviceName, span, p.externalCallDimensions, resourceAttr, extraVals)
		extraDims := map[string]pcommon.Value{
			"address": pcommon.NewValueString(remoteAddr),
		}
		p.externalCallCache(serviceName, span, externalCallKey, resourceAttr, extraDims)
		p.updateExternalCallLatencyMetrics(externalCallKey, latencyInMilliseconds)
	}

	_, dbCallPresent := spanAttr.Get("db.system")
	if span.Kind() != ptrace.SpanKindServer && dbCallPresent {
		dbKey := buildCustomKey(serviceName, span, p.dbCallDimensions, resourceAttr, nil)
		p.dbCache(serviceName, span, dbKey, resourceAttr)
		p.updateDBLatencyMetrics(dbKey, latencyInMilliseconds)
	}

	p.lock.Unlock()
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(key metricKey) {
	p.callSum[key]++
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(key metricKey, latency float64, index int) {
	if _, ok := p.latencyBucketCounts[key]; !ok {
		p.latencyBucketCounts[key] = make([]uint64, len(p.latencyBounds))
	}
	p.latencySum[key] += latency
	p.latencyCount[key]++
	p.latencyBucketCounts[key][index]++
}

// updateDBLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateDBLatencyMetrics(key metricKey, latency float64) {
	p.dbLatencySum[key] += latency
	p.dbLatencyCount[key]++
}

// updateExternalCallLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateExternalCallLatencyMetrics(key metricKey, latency float64) {
	p.externalCallLatencySum[key] += latency
	p.externalCallLatencyCount[key]++
}

// resetAccumulatedMetrics resets the internal maps used to store created metric data. Also purge the cache for
// metricKeyToDimensions.
func (p *processorImp) resetAccumulatedMetrics() {
	p.callSum = make(map[metricKey]int64)

	p.latencyCount = make(map[metricKey]uint64)
	p.externalCallLatencyCount = make(map[metricKey]uint64)
	p.dbLatencyCount = make(map[metricKey]uint64)

	p.latencySum = make(map[metricKey]float64)
	p.dbLatencySum = make(map[metricKey]float64)
	p.externalCallLatencySum = make(map[metricKey]float64)

	p.latencyBucketCounts = make(map[metricKey][]uint64)

	p.metricKeyToDimensions.Purge()
	p.callMetricKeyToDimensions.Purge()
	p.dbMetricKeyToDimensions.Purge()
	p.externalCallMetricKeyToDimensions.Purge()

}

func (p *processorImp) buildCustomDimensionKVs(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map, extraDims map[string]pcommon.Value) pcommon.Map {
	dims := pcommon.NewMap()

	dims.UpsertString(serviceNameKey, serviceName)
	for k, v := range extraDims {
		dims.Upsert(k, v)
	}
	dims.UpsertString(statusCodeKey, span.Status().Code().String())

	for _, d := range optionalDims {

		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)
		if ok {
			dims.Upsert(d.Name, v)
		}
		if foundInResource {
			dims.Upsert(resourcePrefix+d.Name, v)
		}
	}
	return dims
}

// getDimensionValue gets the dimension value for the given configured dimension.
// It searches through the span's attributes first, being the more specific;
// falling back to searching in resource attributes if it can't be found in the span.
// Finally, falls back to the configured default value if provided.
//
// The ok flag indicates if a dimension value was fetched in order to differentiate
// an empty string value from a state where no value was found.
func getDimensionValue(d Dimension, spanAttr pcommon.Map, resourceAttr pcommon.Map) (v pcommon.Value, ok bool, foundInResource bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.Name); exists {
		return attr, true, false
	}

	if attr, exists := resourceAttr.Get(d.Name); exists {
		return attr, false, true
	}

	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.Default != nil {
		return pcommon.NewValueString(*d.Default), true, false
	}
	return v, ok, foundInResource
}

func (p *processorImp) buildDimensionKVs(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map) pcommon.Map {
	dims := pcommon.NewMap()
	dims.UpsertString(serviceNameKey, serviceName)
	dims.UpsertString(operationKey, span.Name())
	dims.UpsertString(spanKindKey, span.Kind().String())
	dims.UpsertString(statusCodeKey, span.Status().Code().String())
	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)
		if ok {
			dims.Upsert(d.Name, v)
		}
		if foundInResource {
			dims.Upsert(resourcePrefix+d.Name, v)
		}
	}
	return dims
}

func concatDimensionValue(metricKeyBuilder *strings.Builder, value string, prefixSep bool) {
	// It's worth noting that from pprof benchmarks, WriteString is the most expensive operation of this processor.
	// Specifically, the need to grow the underlying []byte slice to make room for the appended string.
	if prefixSep {
		metricKeyBuilder.WriteString(metricKeySeparator)
	}
	metricKeyBuilder.WriteString(value)
}

// buildCustomKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// any additional dimensions the user has configured.
// The metric key is a simple concatenation of dimension values.
func buildCustomKey(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map, extraVals []string) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName, false)
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String(), true)

	for _, val := range extraVals {
		concatDimensionValue(&metricKeyBuilder, val, true)
	}

	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)
		if ok {
			concatDimensionValue(&metricKeyBuilder, v.AsString(), true)
		}
		if foundInResource {
			concatDimensionValue(&metricKeyBuilder, resourcePrefix+v.AsString(), true)
		}
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// buildKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func buildKey(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName, false)
	concatDimensionValue(&metricKeyBuilder, span.Name(), true)
	concatDimensionValue(&metricKeyBuilder, span.Kind().String(), true)
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String(), true)

	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)

		if ok {
			concatDimensionValue(&metricKeyBuilder, v.AsString(), true)
		}
		if foundInResource {
			concatDimensionValue(&metricKeyBuilder, resourcePrefix+v.AsString(), true)
		}
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) externalCallCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map, extraDims map[string]pcommon.Value) {
	kvs := p.buildCustomDimensionKVs(serviceName, span, p.externalCallDimensions, resourceAttrs, extraDims)
	p.externalCallMetricKeyToDimensions.ContainsOrAdd(k, kvs)
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) dbCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	p.dbMetricKeyToDimensions.ContainsOrAdd(k, p.buildCustomDimensionKVs(serviceName, span, p.dbCallDimensions, resourceAttrs, nil))
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) callCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	p.callMetricKeyToDimensions.ContainsOrAdd(k, p.buildDimensionKVs(serviceName, span, p.callDimensions, resourceAttrs))
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	p.metricKeyToDimensions.ContainsOrAdd(k, p.buildDimensionKVs(serviceName, span, p.dimensions, resourceAttrs))
}

// copied from prometheus-go-metric-exporter
// sanitize replaces non-alphanumeric characters with underscores in s.
func sanitize(s string) string {
	if len(s) == 0 {
		return s
	}

	// Note: No length limit for label keys because Prometheus doesn't
	// define a length limit, thus we should NOT be truncating label keys.
	// See https://github.com/orijtech/prometheus-go-metrics-exporter/issues/4.
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return s
}

// copied from prometheus-go-metric-exporter
// sanitizeRune converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}
