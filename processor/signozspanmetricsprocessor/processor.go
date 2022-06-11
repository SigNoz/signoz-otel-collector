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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
)

const (
	serviceNameKey             = conventions.AttributeServiceName
	operationKey               = "operation" // is there a constant we can refer to?
	spanKindKey                = "span.kind"
	statusCodeKey              = "status.code"
	TagHTTPStatusCode          = conventions.AttributeHTTPStatusCode
	TagHTTPUrl                 = "http.url"
	metricKeySeparator         = string(byte(0))
	traceIDKey                 = "trace_id"
	defaultDimensionsCacheSize = 1000
	RESOURCE_PREFIX            = "resource_"
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

	dbLatencyCount        map[metricKey]uint64
	dbLatencySum          map[metricKey]float64
	dbLatencyBucketCounts map[metricKey][]uint64

	externalCallLatencyCount        map[metricKey]uint64
	externalCallLatencySum          map[metricKey]float64
	externalCallLatencyBucketCounts map[metricKey][]uint64

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
		{Name: "db.operation"},
	}
	dbCallDimensions = append(dbCallDimensions, pConfig.Dimensions...)

	externalCallDimensions := []Dimension{
		{Name: "http.status_code"},
		{Name: TagHTTPUrl},
		{Name: "http.method"},
	}
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
		dbLatencyBucketCounts:             make(map[metricKey][]uint64),
		externalCallLatencySum:            make(map[metricKey]float64),
		externalCallLatencyCount:          make(map[metricKey]uint64),
		externalCallLatencyBucketCounts:   make(map[metricKey][]uint64),
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
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
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
func (p *processorImp) buildMetrics() (*pdata.Metrics, error) {
	m := pdata.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("signozspanmetricsprocessor")

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
	if p.config.GetAggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
		p.resetAccumulatedMetrics()
	}

	p.lock.Unlock()

	return &m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pdata.InstrumentationLibraryMetrics) error {
	for key := range p.latencyCount {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pdata.MetricDataTypeHistogram)
		mLatency.SetName("signoz_latency")
		mLatency.Histogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.latencyBucketCounts[key])
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

// collectExternalCallMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectDBCallMetrics(ilm pdata.InstrumentationLibraryMetrics) error {
	for key := range p.dbLatencyCount {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pdata.MetricDataTypeHistogram)
		mLatency.SetName("signoz_db_latency")
		mLatency.Histogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.dbLatencyBucketCounts[key])
		dpLatency.SetCount(p.dbLatencyCount[key])
		dpLatency.SetSum(p.dbLatencySum[key])

		dimensions, err := p.getDimensionsByMetricKey(p.dbMetricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectExternalCallMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectExternalCallMetrics(ilm pdata.InstrumentationLibraryMetrics) error {
	for key := range p.externalCallLatencyCount {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pdata.MetricDataTypeHistogram)
		mLatency.SetName("signoz_external_call_latency")
		mLatency.Histogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.externalCallLatencyBucketCounts[key])
		dpLatency.SetCount(p.externalCallLatencyCount[key])
		dpLatency.SetSum(p.externalCallLatencySum[key])

		dimensions, err := p.getDimensionsByMetricKey(p.externalCallMetricKeyToDimensions, key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pdata.InstrumentationLibraryMetrics) error {
	for key := range p.callSum {
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetDataType(pdata.MetricDataTypeSum)
		mCalls.SetName("signoz_calls_total")
		mCalls.Sum().SetIsMonotonic(true)
		mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCalls := mCalls.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
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
func (p *processorImp) getDimensionsByMetricKey(cache *cache.Cache, k metricKey) (*pdata.AttributeMap, error) {
	if item, ok := cache.Get(k); ok {
		if attributeMap, ok := item.(pdata.AttributeMap); ok {
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
func (p *processorImp) aggregateMetrics(traces pdata.Traces) {
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

func (p *processorImp) aggregateMetricsForServiceSpans(rspans pdata.ResourceSpans, serviceName string) {
	ilsSlice := rspans.InstrumentationLibrarySpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			p.aggregateMetricsForSpan(serviceName, span, rspans.Resource().Attributes())
		}
	}
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span pdata.Span, resourceAttr pdata.AttributeMap) {
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
	_, externalCallPresent := spanAttr.Get("http.url")

	if span.Kind() == 3 && externalCallPresent {
		externalCallKey := buildCustomKey(serviceName, span, p.externalCallDimensions, resourceAttr)
		p.externalCallCache(serviceName, span, externalCallKey, resourceAttr)
		p.updateExternalCallLatencyMetrics(externalCallKey, latencyInMilliseconds, index)
	}

	_, dbCallPresent := spanAttr.Get("db.system")
	if span.Kind() == 3 && dbCallPresent {
		dbKey := buildCustomKey(serviceName, span, p.dbCallDimensions, resourceAttr)
		p.dbCache(serviceName, span, dbKey, resourceAttr)
		p.updateDBLatencyMetrics(dbKey, latencyInMilliseconds, index)
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
func (p *processorImp) updateDBLatencyMetrics(key metricKey, latency float64, index int) {
	if _, ok := p.dbLatencyBucketCounts[key]; !ok {
		p.dbLatencyBucketCounts[key] = make([]uint64, len(p.latencyBounds))
	}
	p.dbLatencySum[key] += latency
	p.dbLatencyCount[key]++
	p.dbLatencyBucketCounts[key][index]++
}

// updateExternalCallLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateExternalCallLatencyMetrics(key metricKey, latency float64, index int) {
	if _, ok := p.externalCallLatencyBucketCounts[key]; !ok {
		p.externalCallLatencyBucketCounts[key] = make([]uint64, len(p.latencyBounds))
	}
	p.externalCallLatencySum[key] += latency
	p.externalCallLatencyCount[key]++
	p.externalCallLatencyBucketCounts[key][index]++
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
	p.externalCallLatencyBucketCounts = make(map[metricKey][]uint64)
	p.dbLatencyBucketCounts = make(map[metricKey][]uint64)

	p.metricKeyToDimensions.Purge()
	p.callMetricKeyToDimensions.Purge()
	p.dbMetricKeyToDimensions.Purge()
	p.externalCallMetricKeyToDimensions.Purge()

}

func (p *processorImp) buildCustomDimensionKVs(serviceName string, span pdata.Span, optionalDims []Dimension, resourceAttrs pdata.AttributeMap) pdata.AttributeMap {
	dims := pdata.NewAttributeMap()

	dims.UpsertString(serviceNameKey, serviceName)
	// dims.UpsertString(operationKey, span.Name())
	// dims.UpsertString(spanKindKey, span.Kind().String())
	dims.UpsertString(statusCodeKey, span.Status().Code().String())

	for _, d := range optionalDims {

		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)
		if ok {
			dims.Upsert(d.Name, v)
		}
		if foundInResource {
			dims.Upsert(RESOURCE_PREFIX+d.Name, v)
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
func getDimensionValue(d Dimension, spanAttr pdata.AttributeMap, resourceAttr pdata.AttributeMap) (v pdata.AttributeValue, ok bool, foundInResource bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.Name); exists {
		if d.Name == TagHTTPUrl {

			value := attr.StringVal()
			valueUrl, err := url.Parse(value)
			if err == nil {
				value = valueUrl.Hostname()
			}
			attr = pdata.NewAttributeValueString(value)
		}
		return attr, true, false
	}

	if attr, exists := resourceAttr.Get(d.Name); exists {
		return attr, false, true
	}

	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.Default != nil {
		return pdata.NewAttributeValueString(*d.Default), true, false
	}
	return v, ok, foundInResource
}

func (p *processorImp) buildDimensionKVs(serviceName string, span pdata.Span, optionalDims []Dimension, resourceAttrs pdata.AttributeMap) pdata.AttributeMap {
	dims := pdata.NewAttributeMap()
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
			dims.Upsert(RESOURCE_PREFIX+d.Name, v)
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
func buildCustomKey(serviceName string, span pdata.Span, optionalDims []Dimension, resourceAttrs pdata.AttributeMap) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName, false)
	// concatDimensionValue(&metricKeyBuilder, span.Name(), true)
	// concatDimensionValue(&metricKeyBuilder, span.Kind().String(), true)
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String(), true)

	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValue(d, span.Attributes(), resourceAttrs)
		if ok {
			concatDimensionValue(&metricKeyBuilder, v.AsString(), true)
		}
		if foundInResource {
			concatDimensionValue(&metricKeyBuilder, RESOURCE_PREFIX+v.AsString(), true)
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
func buildKey(serviceName string, span pdata.Span, optionalDims []Dimension, resourceAttrs pdata.AttributeMap) metricKey {
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
			concatDimensionValue(&metricKeyBuilder, RESOURCE_PREFIX+v.AsString(), true)
		}
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) externalCallCache(serviceName string, span pdata.Span, k metricKey, resourceAttrs pdata.AttributeMap) {
	p.externalCallMetricKeyToDimensions.ContainsOrAdd(k, p.buildCustomDimensionKVs(serviceName, span, p.externalCallDimensions, resourceAttrs))
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) dbCache(serviceName string, span pdata.Span, k metricKey, resourceAttrs pdata.AttributeMap) {
	p.dbMetricKeyToDimensions.ContainsOrAdd(k, p.buildCustomDimensionKVs(serviceName, span, p.dbCallDimensions, resourceAttrs))
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) callCache(serviceName string, span pdata.Span, k metricKey, resourceAttrs pdata.AttributeMap) {
	p.callMetricKeyToDimensions.ContainsOrAdd(k, p.buildDimensionKVs(serviceName, span, p.callDimensions, resourceAttrs))
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cache(serviceName string, span pdata.Span, k metricKey, resourceAttrs pdata.AttributeMap) {
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
