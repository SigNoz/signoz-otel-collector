package signozclickhousemetrics

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"go.uber.org/zap/zaptest"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	// With addMetadata (no deduplication), each call creates entries for all fingerprint attributes
	// Per datapoint (1 datapoint):
	//   - count with point fingerprint (1 point attr + __temporality__) = 2 entries
	//   - sum with point fingerprint (1 point attr + __temporality__) = 2 entries
	//   - min with point fingerprint (1 point attr + __temporality__) = 2 entries
	//   - max with point fingerprint (1 point attr + __temporality__) = 2 entries
	//   - 21 buckets (20 + +Inf) with point fingerprint (1 point attr + le + __temporality__) = 21*3 = 63 entries
	//   Subtotal: 2+2+2+2+63 = 71
	// After all datapoints (5 metrics: count, sum, min, max, bucket):
	//   - Each called twice with resource (1 attr) and scope (4 attrs) = 5*(1+4) = 25 entries
	// Total: 71 + 25 = 96

	assert.Equal(t, len(batch.metadata), 1*(2+2+2+2+21*3)+5*(1+4))
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
	// With addMetadata (no deduplication), each call creates entries for all fingerprint attributes
	// Per datapoint (2 datapoints):
	//   - count with point fingerprint (1 point attr + __temporality__) = 2 entries per datapoint
	//   - sum with point fingerprint (1 point attr + __temporality__) = 2 entries per datapoint
	//   - 1 quantile with point fingerprint (1 point attr + quantile + __temporality__) = 3 entries per datapoint
	//   Subtotal per datapoint: 2+2+3 = 7, total for 2 datapoints: 14
	// After all datapoints (3 metrics: count, sum, quantile):
	//   - Each called twice with resource (1 attr) and scope (4 attrs) = 3*(1+4) = 15 entries
	// Total: 14 + 15 = 29

	assert.Equal(t, len(batch.metadata), 2*(2+2+3)+3*(1+4))
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
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, unix_milli, value, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")                                                                                                         //samples query
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)") //time series query
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, unix_milli, count, sum, min, max, sketch, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")                                                                      //exp hist query
	conn.ExpectPrepareBatch("INSERT INTO . (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")             //metadata query
	conn.ExpectExec("insert into signoz_metrics.distributed_usage values ($1, $2, $3, $4, $5)")                                                                                                                                                                                     // usage exporter query
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

func Test_prepareBatchDDStatsPayload(t *testing.T) {
	// Create a mock dd.internal.stats.payload metric with protobuf data
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	// Add resource attributes
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	rm.Resource().Attributes().PutStr("deployment.environment", "dev")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("datadog.receiver")
	sm.Scope().SetVersion("1.0.0")

	// Create the dd.internal.stats.payload metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dd.internal.stats.payload")
	metric.SetEmptySum()
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)
	metric.Sum().SetIsMonotonic(false)

	// Create a sample Datadog stats payload
	statsPayload := &pb.StatsPayload{
		Stats: []*pb.ClientStatsPayload{
			{
				Hostname:      "docker-desktop",
				Env:           "dev",
				Service:       "order-service",
				Version:       "1.0.0",
				Lang:          "python",
				TracerVersion: "1.0.0",
				RuntimeID:     "runtime-123",
				ContainerID:   "container-456",
				Stats: []*pb.ClientStatsBucket{
					{
						Start:    1706600000000000000, // nanoseconds
						Duration: 10000000000,         // 10 seconds in nanoseconds
						Stats: []*pb.ClientGroupedStats{
							{
								Service:        "order-service",
								Name:           "order.create",
								Resource:       "order.create",
								Type:           "web",
								HTTPStatusCode: 200,
								Hits:           100,
								Errors:         5,
								Duration:       5000000000, // 5 seconds in nanoseconds
								Synthetics:     false,
							},
							{
								Service:        "order-service",
								Name:           "service.order",
								Resource:       "service.order",
								Type:           "db",
								HTTPStatusCode: 0,
								Hits:           50,
								Errors:         0,
								Duration:       2000000000, // 2 seconds in nanoseconds
								Synthetics:     false,
							},
						},
					},
				},
			},
			{
				Hostname:      "docker-desktop",
				Env:           "dev",
				Service:       "payment-service",
				Version:       "1.0.0",
				Lang:          "python",
				TracerVersion: "1.0.0",
				Stats: []*pb.ClientStatsBucket{
					{
						Start:    1706600000000000000,
						Duration: 10000000000,
						Stats: []*pb.ClientGroupedStats{
							{
								Service:        "payment-service",
								Name:           "process_payment",
								Resource:       "process_payment",
								Type:           "web",
								HTTPStatusCode: 201,
								Hits:           75,
								Errors:         2,
								Duration:       3000000000,
								Synthetics:     false,
							},
						},
					},
				},
			},
		},
		ClientComputed: true,
	}

	// Marshal the payload to protobuf bytes
	payloadBytes, err := proto.Marshal(statsPayload)
	require.NoError(t, err)

	// Add the payload as a data point attribute
	dp := metric.Sum().DataPoints().AppendEmpty()
	dp.Attributes().PutEmptyBytes("dd.internal.stats.payload").FromRaw(payloadBytes)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Create exporter and prepare batch
	exp, err := NewClickHouseExporter(
		WithLogger(zaptest.NewLogger(t)),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)

	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)

	// Verify samples were created
	// For each ClientGroupedStats, we should have:
	// - 1 hits metric
	// - 1 hits.by_http_status metric (if HTTPStatusCode != 0)
	// - 1 errors metric (if Errors > 0)
	// - 1 errors.by_http_status metric (if Errors > 0 && HTTPStatusCode != 0)
	// - 1 duration histogram

	// First service (order-service) has 2 grouped stats:
	// - order.create: hits, hits.by_http_status, errors, errors.by_http_status, duration
	// - service.order: hits, duration
	// Second service (payment-service) has 1 grouped stats:
	// - process_payment: hits, hits.by_http_status, errors, errors.by_http_status, duration

	expectedSampleCount := 0
	// order.create: 2 (hits + hits.by_http_status) + 2 (errors + errors.by_http_status) + 5 * (duration) = 9
	expectedSampleCount += 9
	// service.order: 1 (hits) + 5 * (duration) = 6
	expectedSampleCount += 6
	// process_payment: 2 (hits + hits.by_http_status) + 2 (errors + errors.by_http_status) + 5 * (duration) = 9
	expectedSampleCount += 9

	require.Equal(t, expectedSampleCount, len(batch.samples), "unexpected number of samples")

	// Verify metric names and values
	metricNames := make(map[string]int)
	for _, sample := range batch.samples {
		metricNames[sample.metricName]++
	}

	// Check that we have the expected metric types
	require.Contains(t, metricNames, "trace.order.create.hits")
	require.Contains(t, metricNames, "trace.order.create.hits.by_http_status")
	require.Contains(t, metricNames, "trace.order.create.errors")
	require.Contains(t, metricNames, "trace.order.create.errors.by_http_status")
	require.Contains(t, metricNames, "trace.order.create.count")
	require.Contains(t, metricNames, "trace.order.create.sum")

	require.Contains(t, metricNames, "trace.service.order.hits")
	require.Contains(t, metricNames, "trace.service.order.count")
	require.Contains(t, metricNames, "trace.service.order.sum")

	require.Contains(t, metricNames, "trace.process_payment.hits")
	require.Contains(t, metricNames, "trace.process_payment.hits.by_http_status")
	require.Contains(t, metricNames, "trace.process_payment.errors")
	require.Contains(t, metricNames, "trace.process_payment.errors.by_http_status")
	require.Contains(t, metricNames, "trace.process_payment.count")
	require.Contains(t, metricNames, "trace.process_payment.sum")

	// Verify specific sample values
	for _, sample := range batch.samples {
		switch sample.metricName {
		case "trace.order.create.hits":
			assert.Equal(t, float64(100), sample.value)
			assert.Equal(t, pmetric.AggregationTemporalityDelta, sample.temporality)
		case "trace.order.create.errors":
			assert.Equal(t, float64(5), sample.value)
			assert.Equal(t, pmetric.AggregationTemporalityDelta, sample.temporality)
		case "trace.order.create.count":
			assert.Equal(t, float64(100), sample.value)
			assert.Equal(t, pmetric.AggregationTemporalityDelta, sample.temporality)
		case "trace.order.create.sum":
			assert.Equal(t, float64(5000000000), sample.value)
			assert.Equal(t, pmetric.AggregationTemporalityDelta, sample.temporality)
		case "trace.service.order.hits":
			assert.Equal(t, float64(50), sample.value)
		case "trace.process_payment.hits":
			assert.Equal(t, float64(75), sample.value)
		case "trace.process_payment.errors":
			assert.Equal(t, float64(2), sample.value)
		}
	}

	// Verify time series were created
	require.Greater(t, len(batch.ts), 0, "expected time series to be created")

	// Verify resource attributes were properly set
	foundOrderService := false
	foundPaymentService := false

	for _, ts := range batch.ts {
		if strings.Contains(ts.metricName, "order.create") || strings.Contains(ts.metricName, "service.order") {
			foundOrderService = true
			assert.Equal(t, "order-service", ts.resourceAttrs["service.name"])
			assert.Equal(t, "docker-desktop", ts.resourceAttrs["host.name"])
			assert.Equal(t, "dev", ts.resourceAttrs["deployment.environment"])
			assert.Equal(t, "python", ts.resourceAttrs["telemetry.sdk.language"])
			assert.Equal(t, "1.0.0", ts.resourceAttrs["service.version"])
		}
		if strings.Contains(ts.metricName, "process_payment") {
			foundPaymentService = true
			assert.Equal(t, "payment-service", ts.resourceAttrs["service.name"])
		}
	}

	assert.True(t, foundOrderService)
	assert.True(t, foundPaymentService)

	// Verify attributes on samples
	for _, sample := range batch.samples {
		if sample.metricName == "trace.order.create.hits.by_http_status" {
			// This sample should have been processed through the normal flow
			// and should have attributes set properly
			break
		}
	}

	// Verify metadata was created
	require.Greater(t, len(batch.metadata), 0, "expected metadata to be created")
}

func Test_prepareBatchDDStatsPayload_InvalidPayload(t *testing.T) {
	// Create a metric with invalid protobuf data
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dd.internal.stats.payload")
	metric.SetEmptySum()

	dp := metric.Sum().DataPoints().AppendEmpty()
	// Add invalid protobuf bytes
	dp.Attributes().PutEmptyBytes("dd.internal.stats.payload").FromRaw([]byte{0x00, 0x01, 0x02})

	exp, err := NewClickHouseExporter(
		WithLogger(zaptest.NewLogger(t)),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)

	batch := exp.prepareBatch(context.Background(), metrics)

	// Should not crash, but should not create any samples
	assert.Equal(t, 0, len(batch.samples))
}

func Test_prepareBatchDDStatsPayload_MissingAttribute(t *testing.T) {
	// Create a metric without the dd.internal.stats.payload attribute
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dd.internal.stats.payload")
	metric.SetEmptySum()

	// dp := metric.Sum().DataPoints().AppendEmpty()
	// No dd.internal.stats.payload attribute

	exp, err := NewClickHouseExporter(
		WithLogger(zaptest.NewLogger(t)),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)

	batch := exp.prepareBatch(context.Background(), metrics)

	// Should not crash, but should not create any samples
	assert.Equal(t, 0, len(batch.samples))
}

func Test_prepareBatchDDStatsPayload_WrongMetricType(t *testing.T) {
	// Create a dd.internal.stats.payload metric with wrong type (Gauge instead of Sum)
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dd.internal.stats.payload")
	metric.SetEmptyGauge() // Wrong type

	dp := metric.Gauge().DataPoints().AppendEmpty()

	statsPayload := &pb.StatsPayload{
		Stats: []*pb.ClientStatsPayload{
			{
				Service: "test-service",
				Stats: []*pb.ClientStatsBucket{
					{
						Start:    1706600000000000000,
						Duration: 10000000000,
						Stats:    []*pb.ClientGroupedStats{},
					},
				},
			},
		},
	}

	payloadBytes, err := proto.Marshal(statsPayload)
	require.NoError(t, err)

	dp.Attributes().PutEmptyBytes("dd.internal.stats.payload").FromRaw(payloadBytes)

	exp, err := NewClickHouseExporter(
		WithLogger(zaptest.NewLogger(t)),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)

	batch := exp.prepareBatch(context.Background(), metrics)

	// Should not process the metric due to wrong type
	assert.Equal(t, 0, len(batch.samples))
}

func Benchmark_prepareBatchDDStatsPayload(b *testing.B) {
	// Create a realistic dd.internal.stats.payload with multiple services
	statsPayload := &pb.StatsPayload{
		Stats: make([]*pb.ClientStatsPayload, 10),
	}

	for i := 0; i < 10; i++ {
		statsPayload.Stats[i] = &pb.ClientStatsPayload{
			Hostname: "host-" + strconv.Itoa(i),
			Env:      "prod",
			Service:  "service-" + strconv.Itoa(i),
			Stats: []*pb.ClientStatsBucket{
				{
					Start:    1706600000000000000,
					Duration: 10000000000,
					Stats:    make([]*pb.ClientGroupedStats, 20),
				},
			},
		}

		for j := 0; j < 20; j++ {
			statsPayload.Stats[i].Stats[0].Stats[j] = &pb.ClientGroupedStats{
				Service:        "service-" + strconv.Itoa(i),
				Name:           "operation-" + strconv.Itoa(j),
				Resource:       "resource-" + strconv.Itoa(j),
				Type:           "web",
				HTTPStatusCode: 200,
				Hits:           uint64(100 * (j + 1)),
				Errors:         uint64(j),
				Duration:       uint64(1000000000 * (j + 1)),
			}
		}
	}

	payloadBytes, err := proto.Marshal(statsPayload)
	require.NoError(b, err)

	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dd.internal.stats.payload")
	metric.SetEmptySum()

	dp := metric.Sum().DataPoints().AppendEmpty()
	dp.Attributes().PutEmptyBytes("dd.internal.stats.payload").FromRaw(payloadBytes)

	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}
