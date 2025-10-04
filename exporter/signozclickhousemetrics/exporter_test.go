package signozclickhousemetrics

import (
	"context"
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
)

func Test_prepareBatchGauge(t *testing.T) {
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "system.memory.usage0",
			unixMilli:   1727286182000,
			value:       0,
		},
	}
	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}

	expectedTs := []ts{
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityUnspecified,
			metricName:    "system.memory.usage0",
			description:   "memory usage of the host",
			unit:          "bytes",
			typ:           pmetric.MetricTypeGauge,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"system.memory.usage0\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Unspecified\",\"gauge.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"__temporality__": "Unspecified", "gauge.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
	}
	assert.Equal(t, len(expectedTs), len(batch.ts))

	for idx, ts := range expectedTs {
		currentTs := batch.ts[idx]

		assert.Equal(t, ts.env, currentTs.env)
		assert.Equal(t, ts.temporality, currentTs.temporality)
		assert.Equal(t, ts.metricName, currentTs.metricName)
		assert.Equal(t, ts.description, currentTs.description)
		assert.Equal(t, ts.unit, currentTs.unit)
		assert.Equal(t, ts.typ, currentTs.typ)
		assert.Equal(t, ts.isMonotonic, currentTs.isMonotonic)
		assert.Equal(t, ts.unixMilli, currentTs.unixMilli)
		assert.Equal(t, ts.labels, currentTs.labels)
		assert.Equal(t, ts.attrs, currentTs.attrs)
		assert.Equal(t, ts.scopeAttrs, currentTs.scopeAttrs)
		assert.Equal(t, ts.resourceAttrs, currentTs.resourceAttrs)
	}
}

func Test_prepareBatchSum(t *testing.T) {
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "system.cpu.time0",
			unixMilli:   1727286182000,
			value:       0,
		},
	}
	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}

	expectedTs := []ts{
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "system.cpu.time0",
			description:   "cpu time of the host",
			unit:          "s",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   true,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"system.cpu.time0\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Cumulative\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\",\"sum.attr_0\":\"1\"}",
			attrs:         map[string]string{"__temporality__": "Cumulative", "sum.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
	}
	assert.Equal(t, len(expectedTs), len(batch.ts))

	for idx, ts := range expectedTs {
		currentTs := batch.ts[idx]

		assert.Equal(t, ts.env, currentTs.env)
		assert.Equal(t, ts.temporality, currentTs.temporality)
		assert.Equal(t, ts.metricName, currentTs.metricName)
		assert.Equal(t, ts.description, currentTs.description)
		assert.Equal(t, ts.unit, currentTs.unit)
		assert.Equal(t, ts.typ, currentTs.typ)
		assert.Equal(t, ts.isMonotonic, currentTs.isMonotonic)
		assert.Equal(t, ts.unixMilli, currentTs.unixMilli)
		assert.Equal(t, ts.labels, currentTs.labels)
		assert.Equal(t, ts.attrs, currentTs.attrs)
		assert.Equal(t, ts.scopeAttrs, currentTs.scopeAttrs)
		assert.Equal(t, ts.resourceAttrs, currentTs.resourceAttrs)
	}
}

func Test_prepareBatchHistogram(t *testing.T) {
	metrics := pmetricsgen.GenerateHistogramMetrics(1, 1, 1, 1, 1, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	// there should be 4 (count, sum, min, max) + 20 (for each bucket) + 1 (for the inf bucket) = 25 samples
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.count",
			unixMilli:   1727286182000,
			value:       30,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.sum",
			unixMilli:   1727286182000,
			value:       35,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration0.min",
			unixMilli:   1727286182000,
			value:       0,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration0.max",
			unixMilli:   1727286182000,
			value:       12,
		},
	}
	cumulativeCount := 0
	// 20 buckets
	for i := 0; i < 20; i++ {
		cumulativeCount += 1
		if i == 5 || i == 12 {
			cumulativeCount += i - 1
		}
		expectedSamples = append(expectedSamples, sample{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.bucket",
			unixMilli:   1727286182000,
			value:       float64(cumulativeCount),
		})
	}

	// 1 for the inf bucket
	expectedSamples = append(expectedSamples, sample{
		env:         "",
		temporality: pmetric.AggregationTemporalityCumulative,
		metricName:  "http.server.duration0.bucket",
		unixMilli:   1727286182000,
		value:       30,
	})

	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}

	// 4 ts for count, sum, min, max
	expectedTs := []ts{
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.count",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.count\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Cumulative\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"__temporality__": "Cumulative", "histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.sum",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.sum\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityUnspecified,
			metricName:    "http.server.duration0.min",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeGauge,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.min\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityUnspecified,
			metricName:    "http.server.duration0.max",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeGauge,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.max\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
	}

	// 20 buckets, one separate ts for each bucket
	for i := 0; i < 20; i++ {
		expectedTs = append(expectedTs, ts{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.bucket",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeHistogram,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        fmt.Sprintf("{\"__name__\":\"http.server.duration0.bucket\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"le\":\"%d\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}", i),
			attrs:         map[string]string{"histogram.attr_0": "1", "le": strconv.Itoa(i)},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		})
	}

	// add le=+Inf sample
	expectedTs = append(expectedTs, ts{
		env:           "",
		temporality:   pmetric.AggregationTemporalityCumulative,
		metricName:    "http.server.duration0.bucket",
		description:   "server duration of the http server",
		unit:          "ms",
		typ:           pmetric.MetricTypeHistogram,
		isMonotonic:   false,
		unixMilli:     1727286182000,
		labels:        "{\"__name__\":\"http.server.duration0.bucket\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"le\":\"+Inf\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
		attrs:         map[string]string{"histogram.attr_0": "1", "le": "+Inf"},
		scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
		resourceAttrs: map[string]string{"resource.attr_0": "value0"},
	})

	for idx, ts := range expectedTs {
		currentTs := batch.ts[idx]

		assert.Equal(t, ts.env, currentTs.env)
		assert.Equal(t, ts.temporality, currentTs.temporality)
		assert.Equal(t, ts.metricName, currentTs.metricName)
		assert.Equal(t, ts.description, currentTs.description)
		assert.Equal(t, ts.unit, currentTs.unit)
		assert.Equal(t, ts.typ, currentTs.typ)
	}

	// metadata
	// count, sum, min, max
	// 1 => resource attr
	// 4 => scope attr + __scope.version__ + __scope.schema_url__ + __scope.name__
	// 2 => point attr + __temporality__
	// bucket
	// 1 => resource attr
	// 4 => scope attr + __scope.version__ + __scope.schema_url__ + __scope.name__
	// 23 => point attr + __temporality__ + 21 buckets

	assert.Equal(t, len(batch.metadata), 4*(2+4+1)+1*(1+4+23))
	for _, item := range batch.metadata {
		validSuffix := false
		if strings.HasSuffix(item.metricName, countSuffix) || strings.HasSuffix(item.metricName, sumSuffix) {
			validSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityCumulative)
			assert.Equal(t, item.typ, pmetric.MetricTypeSum)
		}
		if strings.HasSuffix(item.metricName, bucketSuffix) {
			validSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityCumulative)
			assert.Equal(t, item.typ, pmetric.MetricTypeHistogram)
		}
		if strings.HasSuffix(item.metricName, minSuffix) || strings.HasSuffix(item.metricName, maxSuffix) {
			validSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityUnspecified)
			assert.Equal(t, item.typ, pmetric.MetricTypeGauge)
		}
		assert.True(t, validSuffix)
	}
}

func Test_prepareBatchExponentialHistogram(t *testing.T) {
	metrics := pmetricsgen.GenerateExponentialHistogramMetrics(2, 1, 1, 1, 1, 22, 0, 0)
	exp, err := NewClickHouseExporter(
		WithEnableExpHist(true),
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)

	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1.count",
			unixMilli:   1727286182000,
			value:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1.sum",
			unixMilli:   1727286182000,
			value:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration1.min",
			unixMilli:   1727286182000,
			value:       0,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration1.max",
			unixMilli:   1727286182000,
			value:       1,
		},
	}

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}

	expectedExpHistSamples := []exponentialHistogramSample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1",
			unixMilli:   1727286182000,
			sketch: chproto.DD{
				Mapping: &chproto.IndexMapping{Gamma: math.Pow(2, math.Pow(2, float64(-2)))},
				PositiveValues: &chproto.Store{
					ContiguousBinIndexOffset: 1,
					ContiguousBinCounts:      []float64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10},
				},
				NegativeValues: &chproto.Store{
					ContiguousBinIndexOffset: 1,
					ContiguousBinCounts:      []float64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10},
				},
				ZeroCount: 0,
			},
		},
	}

	for idx, sample := range expectedExpHistSamples {
		curSample := batch.expHist[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.sketch, curSample.sketch)
	}

	// metadata
	// count, sum, min, max
	// 1 => resource attr
	// 4 => scope attr + __scope.version__ + __scope.schema_url__ + __scope.name__
	// 2 => point attr + __temporality__
	// sketch
	// 2 => point attr + __temporality__

	assert.Equal(t, len(batch.metadata), 4*(1+4+2)+(2))
	for _, item := range batch.metadata {
		metaSuffix := false
		if strings.HasSuffix(item.metricName, countSuffix) || strings.HasSuffix(item.metricName, sumSuffix) {
			metaSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityDelta)
			assert.Equal(t, item.typ, pmetric.MetricTypeSum)
		}
		if strings.HasSuffix(item.metricName, minSuffix) || strings.HasSuffix(item.metricName, maxSuffix) {
			metaSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityUnspecified)
			assert.Equal(t, item.typ, pmetric.MetricTypeGauge)
		}
		if !metaSuffix {
			metaSuffix = true
			assert.Equal(t, item.typ, pmetric.MetricTypeExponentialHistogram)
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityDelta)
		}
		assert.True(t, metaSuffix)
	}
}

func Test_prepareBatchSummary(t *testing.T) {
	metrics := pmetricsgen.GenerateSummaryMetrics(1, 2, 1, 1, 1, 1, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)

	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.count",
			unixMilli:   1727286182000,
			value:       0,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.sum",
			unixMilli:   1727286182000,
			value:       0,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.quantile",
			unixMilli:   1727286182000,
			value:       0,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.count",
			unixMilli:   1727286183000,
			value:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.sum",
			unixMilli:   1727286183000,
			value:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.quantile",
			unixMilli:   1727286183000,
			value:       1,
		},
	}

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}

	// metadata
	// count, sum
	// 1 => resource attr
	// 4 => scope attr + __scope.version__ + __scope.schema_url__ + __scope.name__
	// 2 => point attr + __temporality__
	// bucket
	// 1 => resource attr
	// 4 => scope attr + __scope.version__ + __scope.schema_url__ + __scope.name__
	// 3 => point attr + __temporality__ + 1 quantile

	assert.Equal(t, len(batch.metadata), 2*(1+4+2)+1*(1+4+3))
	for _, item := range batch.metadata {
		validSuffix := false
		if strings.HasSuffix(item.metricName, countSuffix) || strings.HasSuffix(item.metricName, sumSuffix) {
			validSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityCumulative)
			assert.Equal(t, item.typ, pmetric.MetricTypeSum)
		}
		if strings.HasSuffix(item.metricName, quantilesSuffix) {
			validSuffix = true
			assert.Equal(t, item.temporality, pmetric.AggregationTemporalityCumulative)
			assert.Equal(t, item.typ, pmetric.MetricTypeSummary)
		}
		assert.True(t, validSuffix)
	}
}

func Benchmark_prepareBatchGauge(b *testing.B) {
	// 10k gauge metrics * 10 data points = 100k data point in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateGaugeMetrics(10000, 10, 10, 10, 10, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Benchmark_prepareBatchSum(b *testing.B) {
	// 10k sum * 10 data points = 100k data point in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateSumMetrics(10000, 10, 10, 10, 10, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Benchmark_prepareBatchHistogram(b *testing.B) {
	// 1k histogram * 10 datapoints * 20 buckets = 200k samples in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateHistogramMetrics(1000, 10, 10, 10, 10, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Benchmark_prepareBatchExponentialHistogram(b *testing.B) {
	// 1k histogram * 10 datapoints * (20 positive + 20 negative) buckets = 400k samples in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateExponentialHistogramMetrics(10000, 10, 10, 10, 10, 0, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(
		WithEnableExpHist(true),
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Benchmark_prepareBatchSummary(b *testing.B) {
	// 10k summary * 10 datapoints = 100k+ samples in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateSummaryMetrics(10000, 10, 10, 10, 10, 0, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Test_prepareBatchGaugeWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateGaugeMetrics(2, 5, 7, 9, 2, 5, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.Equal(t, 0, len(batch.samples))
	assert.Equal(t, 0, len(batch.ts))
}

func Test_prepareBatchGaugeWithStaleNan(t *testing.T) {
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 1)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "system.memory.usage0",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
	}
	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
		assert.Equal(t, sample.flags, curSample.flags)
	}
}

func Test_prepareBatchHistogramWithNoRecordedValue(t *testing.T) {
	metrics := pmetricsgen.GenerateHistogramMetrics(1, 1, 1, 1, 1, 0, 1)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	// there should be 4 (count, sum, min, max) + 20 (for each bucket) + 1 (for the inf bucket) = 25 samples
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.count",
			unixMilli:   1727286182000,
			value:       30,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.sum",
			unixMilli:   1727286182000,
			value:       35,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration0.min",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration0.max",
			unixMilli:   1727286182000,
			value:       12,
			flags:       1,
		},
	}
	cumulativeCount := 0
	// 20 buckets
	for i := 0; i < 20; i++ {
		cumulativeCount += 1
		if i == 5 || i == 12 {
			cumulativeCount += i - 1
		}
		expectedSamples = append(expectedSamples, sample{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "http.server.duration0.bucket",
			unixMilli:   1727286182000,
			value:       float64(cumulativeCount),
			flags:       1,
		})
	}

	// 1 for the inf bucket
	expectedSamples = append(expectedSamples, sample{
		env:         "",
		temporality: pmetric.AggregationTemporalityCumulative,
		metricName:  "http.server.duration0.bucket",
		unixMilli:   1727286182000,
		value:       30,
		flags:       1,
	})

	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
		assert.Equal(t, sample.flags, curSample.flags)
	}

	// 4 ts for count, sum, min, max
	expectedTs := []ts{
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.count",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.count\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Cumulative\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"__temporality__": "Cumulative", "histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.sum",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.sum\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityUnspecified,
			metricName:    "http.server.duration0.min",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeGauge,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.min\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityUnspecified,
			metricName:    "http.server.duration0.max",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeGauge,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"http.server.duration0.max\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
			attrs:         map[string]string{"histogram.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
	}

	// 20 buckets, one separate ts for each bucket
	for i := 0; i < 20; i++ {
		expectedTs = append(expectedTs, ts{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "http.server.duration0.bucket",
			description:   "server duration of the http server",
			unit:          "ms",
			typ:           pmetric.MetricTypeHistogram,
			isMonotonic:   false,
			unixMilli:     1727286182000,
			labels:        fmt.Sprintf("{\"__name__\":\"http.server.duration0.bucket\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"le\":\"%d\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}", i),
			attrs:         map[string]string{"histogram.attr_0": "1", "le": strconv.Itoa(i)},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		})
	}

	// add le=+Inf sample
	expectedTs = append(expectedTs, ts{
		env:           "",
		temporality:   pmetric.AggregationTemporalityCumulative,
		metricName:    "http.server.duration0.bucket",
		description:   "server duration of the http server",
		unit:          "ms",
		typ:           pmetric.MetricTypeHistogram,
		isMonotonic:   false,
		unixMilli:     1727286182000,
		labels:        "{\"__name__\":\"http.server.duration0.bucket\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"le\":\"+Inf\",\"histogram.attr_0\":\"1\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\"}",
		attrs:         map[string]string{"histogram.attr_0": "1", "le": "+Inf"},
		scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
		resourceAttrs: map[string]string{"resource.attr_0": "value0"},
	})

	for idx, ts := range expectedTs {
		currentTs := batch.ts[idx]

		assert.Equal(t, ts.env, currentTs.env)
		assert.Equal(t, ts.temporality, currentTs.temporality)
		assert.Equal(t, ts.metricName, currentTs.metricName)
		assert.Equal(t, ts.description, currentTs.description)
		assert.Equal(t, ts.unit, currentTs.unit)
		assert.Equal(t, ts.typ, currentTs.typ)
	}
}

func Test_prepareBatchHistogramWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateHistogramMetrics(1, 1, 1, 1, 1, 1, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.Equal(t, 0, len(batch.samples))

}

func Test_prepareBatchSumWithNoRecordedValue(t *testing.T) {
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 1, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "system.cpu.time0",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
	}
	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
		assert.Equal(t, sample.flags, curSample.flags)
	}

	expectedTs := []ts{
		{
			env:           "",
			temporality:   pmetric.AggregationTemporalityCumulative,
			metricName:    "system.cpu.time0",
			description:   "cpu time of the host",
			unit:          "s",
			typ:           pmetric.MetricTypeSum,
			isMonotonic:   true,
			unixMilli:     1727286182000,
			labels:        "{\"__name__\":\"system.cpu.time0\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Cumulative\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\",\"sum.attr_0\":\"1\"}",
			attrs:         map[string]string{"__temporality__": "Cumulative", "sum.attr_0": "1"},
			scopeAttrs:    map[string]string{"__scope.name__": "go.signoz.io/app/reader", "__scope.schema_url__": "scope.schema_url", "__scope.version__": "1.0.0", "scope.attr_0": "value0"},
			resourceAttrs: map[string]string{"resource.attr_0": "value0"},
		},
	}
	assert.Equal(t, len(expectedTs), len(batch.ts))

	for idx, ts := range expectedTs {
		currentTs := batch.ts[idx]

		assert.Equal(t, ts.env, currentTs.env)
		assert.Equal(t, ts.temporality, currentTs.temporality)
		assert.Equal(t, ts.metricName, currentTs.metricName)
		assert.Equal(t, ts.description, currentTs.description)
		assert.Equal(t, ts.unit, currentTs.unit)
		assert.Equal(t, ts.typ, currentTs.typ)
		assert.Equal(t, ts.isMonotonic, currentTs.isMonotonic)
		assert.Equal(t, ts.unixMilli, currentTs.unixMilli)
		assert.Equal(t, ts.labels, currentTs.labels)
		assert.Equal(t, ts.attrs, currentTs.attrs)
		assert.Equal(t, ts.scopeAttrs, currentTs.scopeAttrs)
		assert.Equal(t, ts.resourceAttrs, currentTs.resourceAttrs)
	}
}

func Test_prepareBatchSumWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 0, 1)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.Equal(t, 0, len(batch.samples))

}

func Test_prepareBatchSummaryWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateSummaryMetrics(1, 2, 1, 1, 1, 1, 2, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	assert.Equal(t, 0, len(batch.samples))
}

func Test_prepareBatchSummaryWithNoRecordedValue(t *testing.T) {
	metrics := pmetricsgen.GenerateSummaryMetrics(1, 2, 1, 1, 1, 1, 0, 2)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)

	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.count",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.sum",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.quantile",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.count",
			unixMilli:   1727286183000,
			value:       1,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.sum",
			unixMilli:   1727286183000,
			value:       1,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "zk.duration0.quantile",
			unixMilli:   1727286183000,
			value:       1,
			flags:       1,
		},
	}

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
		assert.Equal(t, sample.flags, curSample.flags)
	}
}

func Test_prepareBatchExponentialHistogramWithNoRecordedValue(t *testing.T) {
	metrics := pmetricsgen.GenerateExponentialHistogramMetrics(2, 1, 1, 1, 1, 22, 0, 1)
	exp, err := NewClickHouseExporter(
		WithEnableExpHist(true),
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)

	expectedSamples := []sample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1.count",
			unixMilli:   1727286182000,
			value:       1,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1.sum",
			unixMilli:   1727286182000,
			value:       1,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration1.min",
			unixMilli:   1727286182000,
			value:       0,
			flags:       1,
		},
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityUnspecified,
			metricName:  "http.server.duration1.max",
			unixMilli:   1727286182000,
			value:       1,
			flags:       1,
		},
	}

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
		assert.Equal(t, sample.flags, curSample.flags)
	}

	expectedExpHistSamples := []exponentialHistogramSample{
		{
			env:         "",
			temporality: pmetric.AggregationTemporalityDelta,
			metricName:  "http.server.duration1",
			unixMilli:   1727286182000,
			flags:       1,
			sketch: chproto.DD{
				Mapping: &chproto.IndexMapping{Gamma: math.Pow(2, math.Pow(2, float64(-2)))},
				PositiveValues: &chproto.Store{
					ContiguousBinIndexOffset: 1,
					ContiguousBinCounts:      []float64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10},
				},
				NegativeValues: &chproto.Store{
					ContiguousBinIndexOffset: 1,
					ContiguousBinCounts:      []float64{0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 11, 1, 1, 1, 1, 10},
				},
				ZeroCount: 0,
			},
		},
	}

	for idx, sample := range expectedExpHistSamples {
		curSample := batch.expHist[idx]
		assert.Equal(t, sample.env, curSample.env)
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.sketch, curSample.sketch)
		assert.Equal(t, sample.flags, curSample.flags)
	}
}

func Test_prepareBatchExponentialHistogramWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateExponentialHistogramMetrics(2, 1, 1, 1, 1, 22, 1, 0)
	exp, err := NewClickHouseExporter(
		WithEnableExpHist(true),
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	assert.Equal(t, 0, len(batch.samples))
}

func Test_shutdown(t *testing.T) {
	conn, err := cmock.NewClickHouseNative(nil)
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	id := uuid.New()
	logger := zaptest.NewLogger(t)
	conn.MatchExpectationsInOrder(false)
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, unix_milli, value, flags) VALUES (?, ?, ?, ?, ?, ?, ?)")                                                                                                                        //samples query
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")                //time series query
	conn.ExpectPrepareBatch("INSERT INTO . (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)") //metadata query
	conn.ExpectExec("insert into signoz_metrics.distributed_usage values ($1, $2, $3, $4, $5)")                                                                                                                                                                         // usage exporter query
	conn.ExpectClose()
	usageCollector := usage.NewUsageCollector(
		id,
		conn,
		usage.Options{
			ReportingInterval: 5 * time.Minute,
		},
		"signoz_metrics",
		UsageExporter, logger)
	chExporter, err := NewClickHouseExporter(
		WithConn(conn),
		WithUsageCollector(usageCollector),
		WithExporterID(id),
		WithEnableExpHist(true),
		WithLogger(logger),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	if err != nil {
		log.Fatalf("an error '%s' was not expected when creating new exporter", err)
	}

	// Send one metric before shutdown
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	err = chExporter.PushMetrics(context.Background(), metrics)
	if err != nil {
		t.Fatalf("unexpected error pushing metrics: %v", err)
	}

	wg := new(sync.WaitGroup)
	err = chExporter.Shutdown(context.Background())
	if err != nil {
		log.Fatalf("an error '%s' was not expected when shutting down exporter", err)
	}
	errChan := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- chExporter.PushMetrics(context.Background(), pmetric.NewMetrics())
		}()
	}
	wg.Wait()
	close(errChan)
	for ok := range errChan {
		assert.Error(t, ok)
	}
}

func Test_writeBatch_LogsLateSamplesAndMaxLate(t *testing.T) {
	// Prepare a mock ClickHouse connection and allow any order of expectations.
	conn, err := cmock.NewClickHouseNative(nil)
	require.NoError(t, err)
	conn.MatchExpectationsInOrder(false)
	// We expect the three batches used by PushMetrics (samples, timeseries, metadata).
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, unix_milli, value, flags) VALUES (?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectPrepareBatch("INSERT INTO . (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

	// Capture logs.
	core, obs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	exp, err := NewClickHouseExporter(
		WithConn(conn),
		WithLogger(logger),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)

	// Generate one gauge metric with an old timestamp (generator uses fixed past ts),
	// which should be considered "late" against time.Now() in writeSamples().
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)

	// Push and ensure no error from pipeline.
	ctx := context.Background()
	require.NoError(t, exp.PushMetrics(ctx, metrics))

	// Assert we logged lateSamples with the metric present and count == 1.
	lateEntries := obs.FilterMessage("number of late samples by metric").All()
	require.Len(t, lateEntries, 1, "expected exactly one lateSamples log entry")
	var late map[string]int64
	for _, f := range lateEntries[0].Context {
		if f.Key == "lateSamples" {
			// observer stores zap.Any() as a reflect field; extract as map[string]int64
			if m, ok := f.Interface.(map[string]int64); ok {
				late = m
			}
		}
	}
	require.NotNil(t, late, "lateSamples should be present in the log fields")
	// Metric name from the generator used elsewhere in tests.
	require.Equal(t, int64(1), late["system.memory.usage0"])

	// Assert we logged maxLate and that it’s >= 60_000 ms for the same metric.
	maxLateEntries := obs.FilterMessage("max lateness by metric (ms)").All()
	require.Len(t, maxLateEntries, 1, "expected exactly one maxLate log entry")
	var maxLate map[string]int64
	for _, f := range maxLateEntries[0].Context {
		if f.Key == "maxLate" {
			if m, ok := f.Interface.(map[string]int64); ok {
				maxLate = m
			}
		}
	}
	require.NotNil(t, maxLate, "maxLate should be present in the log fields")
	// We can't assert an exact value (depends on wall clock), but it must be >= 60s.
	require.GreaterOrEqual(t, maxLate["system.memory.usage0"], int64(60000))

	// Assert we logged the minute-bucket distribution for late samples.
	bucketEntries := obs.FilterMessage("late samples buckets (minutes)").All()
	require.Len(t, bucketEntries, 1, "expected exactly one minute-bucket log entry")

	var perMetric map[string][]int64
	var globalBuckets []int64
	for _, f := range bucketEntries[0].Context {
		switch f.Key {
		case "per_metric":
			pm, ok := coercePerMetricField(f)
			if ok {
				perMetric = pm
			}
		case "global":
			gb, ok := coerceInt64SliceField(f)
			if ok {
				globalBuckets = gb
			}
		}
	}
	require.NotNil(t, perMetric, "per_metric buckets should be present in the log fields")
	require.NotNil(t, globalBuckets, "global buckets should be present in the log fields")

	// Check per-metric distribution for the generated metric.
	pm, ok := perMetric["system.memory.usage0"]
	require.True(t, ok, "per_metric must include system.memory.usage0")
	require.Len(t, pm, 10, "per_metric slice must have 10 buckets")
	// Because the datapoint timestamp is far in the past, it must land in the ≥10m bucket (index 9).
	require.Equal(t, int64(1), pm[9], "expected one sample in ≥10m bucket")
	for i := 0; i < 9; i++ {
		require.Equalf(t, int64(0), pm[i], "expected bucket %d to be zero", i)
	}

	// Global distribution should mirror the single late sample as well.
	require.Len(t, globalBuckets, 10, "global bucket slice must have 10 buckets")
	require.Equal(t, int64(1), globalBuckets[9], "expected one global sample in ≥10m bucket")
	for i := 0; i < 9; i++ {
		require.Equalf(t, int64(0), globalBuckets[i], "expected global bucket %d to be zero", i)
	}
}

// --- helpers robust to zap field storage ---

// coerceInt64SliceField extracts []int64 from either Int64sType or ReflectType fields.
func coerceInt64SliceField(f zapcore.Field) ([]int64, bool) {
	switch f.Type {
	case zapcore.ArrayMarshalerType:
		if s, ok := f.Interface.([]int64); ok {
			return s, true
		}
		// Defensive: sometimes Interface could be a reflect.Value wrapping []int64
		return coerceInt64SliceAny(f.Interface)
	case zapcore.ReflectType:
		return coerceInt64SliceAny(f.Interface)
	default:
		return nil, false
	}
}

// coercePerMetricField extracts map[string][]int64 from a ReflectType field.
func coercePerMetricField(f zapcore.Field) (map[string][]int64, bool) {
	if f.Type != zapcore.ReflectType {
		// Some zap cores may still store it as a plain interface map.
		if m, ok := f.Interface.(map[string][]int64); ok {
			return m, true
		}
	}
	rv := reflect.ValueOf(f.Interface)
	if rv.Kind() != reflect.Map {
		return nil, false
	}
	out := make(map[string][]int64, rv.Len())
	for _, mk := range rv.MapKeys() {
		if mk.Kind() != reflect.String {
			return nil, false
		}
		key := mk.String()
		mv := rv.MapIndex(mk)
		if !mv.IsValid() {
			continue
		}
		slice, ok := coerceInt64SliceAny(mv.Interface())
		if !ok {
			// attempt with reflect.Value path
			if s, ok2 := coerceInt64SliceValue(mv); ok2 {
				slice = s
				ok = true
			}
		}
		if !ok {
			return nil, false
		}
		out[key] = slice
	}
	return out, true
}

// coerceInt64SliceAny handles []int64, []any, []interface{}, []float64, []int via reflection.
func coerceInt64SliceAny(v any) ([]int64, bool) {
	if s, ok := v.([]int64); ok {
		return s, true
	}
	if s, ok := v.([]any); ok {
		out := make([]int64, len(s))
		for i, el := range s {
			n, ok := toInt64(el)
			if !ok {
				return nil, false
			}
			out[i] = n
		}
		return out, true
	}
	// Fall back to reflect
	return coerceInt64SliceValue(reflect.ValueOf(v))
}

func coerceInt64SliceValue(rv reflect.Value) ([]int64, bool) {
	if !rv.IsValid() {
		return nil, false
	}
	if rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	n := rv.Len()
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		n64, ok := toInt64(rv.Index(i).Interface())
		if !ok {
			return nil, false
		}
		out[i] = n64
	}
	return out, true
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	case uint64:
		return int64(x), true
	case uint:
		return int64(x), true
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64 {
			return rv.Int(), true
		}
		if rv.IsValid() && rv.Kind() >= reflect.Uint && rv.Kind() <= reflect.Uint64 {
			return int64(rv.Uint()), true
		}
		return 0, false
	}
}
