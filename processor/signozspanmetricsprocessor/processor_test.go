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
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"

	"github.com/SigNoz/signoz-otel-collector/processor/signozspanmetricsprocessor/internal/cache"
	genmetadata "github.com/SigNoz/signoz-otel-collector/processor/signozspanmetricsprocessor/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/processor/signozspanmetricsprocessor/mocks"
)

const (
	stringAttrName         = "stringAttrName"
	intAttrName            = "intAttrName"
	doubleAttrName         = "doubleAttrName"
	boolAttrName           = "boolAttrName"
	nullAttrName           = "nullAttrName"
	mapAttrName            = "mapAttrName"
	arrayAttrName          = "arrayAttrName"
	notInSpanAttrName0     = "shouldBeInMetric"
	notInSpanAttrName1     = "shouldNotBeInMetric"
	regionResourceAttrName = "region"
	conflictResourceAttr   = "host.name"
	DimensionsCacheSize    = 2

	sampleRegion          = "us-east-1"
	sampleConflictingHost = "conflicting-host"
	sampleLatency         = float64(11)
	sampleLatencyDuration = time.Duration(sampleLatency) * time.Millisecond
)

var (
	testID = "test-instance-id"
)

// metricID represents the minimum attributes that uniquely identifies a metric in our tests.
type metricID struct {
	service    string
	operation  string
	kind       string
	statusCode string
}

type metricDataPoint interface {
	Attributes() pcommon.Map
}

type serviceSpans struct {
	serviceName string
	spans       []span
}

type span struct {
	operation  string
	kind       ptrace.SpanKind
	statusCode ptrace.StatusCode
}

func TestProcessorStart(t *testing.T) {
	// Create otlp exporters.
	componentID, mexp, texp := newOTLPExporters(t)

	for _, tc := range []struct {
		name            string
		exporter        component.Component
		metricsExporter string
		wantErrorMsg    string
	}{
		{"export to active otlp metrics exporter", mexp, "otlp", ""},
		{"unable to find configured exporter in active exporter list", mexp, "prometheus", "failed to find metrics exporter: 'prometheus'; please configure metrics_exporter from one of: [otlp]"},
		{"export to active otlp traces exporter should error", texp, "otlp", "the exporter \"otlp\" isn't a metrics exporter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			exporters := map[pipeline.Signal]map[component.ID]component.Component{
				pipeline.SignalMetrics: {
					componentID: tc.exporter,
				},
			}
			mhost := &mocks.Host{}
			mhost.On("GetExporters").Return(exporters)

			// Create spanmetrics processor
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.MetricsExporter = tc.metricsExporter

			procCreationParams := processortest.NewNopSettings(genmetadata.Type)
			traceProcessor, err := factory.CreateTraces(context.Background(), procCreationParams, cfg, consumertest.NewNop())
			require.NoError(t, err)

			// Test
			smp := traceProcessor.(*processorImp)
			err = smp.Start(context.Background(), mhost)

			// Verify
			if tc.wantErrorMsg != "" {
				assert.EqualError(t, err, tc.wantErrorMsg)
			} else {
				assert.NoError(t, err)

				shutdownErr := smp.Shutdown(context.Background())
				assert.NoError(t, shutdownErr)
			}
		})
	}
}

func TestProcessorShutdown(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), testID, cfg, nil)
	p.tracesConsumer = next
	assert.NoError(t, err)
	err = p.Shutdown(context.Background())

	// Verify
	assert.NoError(t, err)
}

func TestConfigureLatencyBounds(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.LatencyHistogramBuckets = []time.Duration{
		3 * time.Nanosecond,
		3 * time.Microsecond,
		3 * time.Millisecond,
		3 * time.Second,
	}

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), testID, cfg, nil)
	p.tracesConsumer = next

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, []float64{0.000003, 0.003, 3, 3000}, p.latencyBounds)
}

func TestProcessorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), testID, cfg, nil)
	p.tracesConsumer = next
	assert.NoError(t, err)
	caps := p.Capabilities()

	// Verify
	assert.NotNil(t, p)
	assert.Equal(t, false, caps.MutatesData)
}

func TestProcessorConsumeTracesErrors(t *testing.T) {
	for _, tc := range []struct {
		name              string
		consumeMetricsErr error
		consumeTracesErr  error
	}{
		{
			name: "ConsumeMetrics error",
		},
		{
			name:             "ConsumeTraces error",
			consumeTracesErr: fmt.Errorf("consume traces error"),
		},
		{
			name:             "ConsumeMetrics and ConsumeTraces error",
			consumeTracesErr: fmt.Errorf("consume traces error"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			logger := zap.NewNop()

			mexp := &mocks.MetricsExporter{}
			mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(tc.consumeMetricsErr)

			tcon := &mocks.TracesConsumer{}
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(tc.consumeTracesErr)

			p := newProcessorImp(mexp, tcon, nil, cumulative, logger, []ExcludePattern{})

			traces := buildSampleTrace()

			// Test
			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err := p.ConsumeTraces(ctx, traces)

			switch {
			case tc.consumeMetricsErr != nil && tc.consumeTracesErr != nil:
				assert.EqualError(t, err, tc.consumeMetricsErr.Error()+"; "+tc.consumeTracesErr.Error())
			case tc.consumeMetricsErr != nil:
				assert.EqualError(t, err, tc.consumeMetricsErr.Error())
			case tc.consumeTracesErr != nil:
				assert.EqualError(t, err, tc.consumeTracesErr.Error())
			}
		})
	}
}

func TestProcessorConsumeTraces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name                   string
		aggregationTemporality string
		verifier               func(t testing.TB, input pmetric.Metrics) bool
		traces                 []ptrace.Traces
	}{
		{
			name:                   "Test single consumption, three spans (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyConsumeMetricsInputCumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			name:                   "Test single consumption, three spans (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
		},
		{
			// More consumptions, should accumulate additively.
			name:                   "Test two consumptions (Cumulative).",
			aggregationTemporality: cumulative,
			verifier:               verifyMultipleCumulativeConsumptions(),
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// More consumptions, should not accumulate. Therefore, end state should be the same as single consumption case.
			name:                   "Test two consumptions (Delta).",
			aggregationTemporality: delta,
			verifier:               verifyConsumeMetricsInputDelta,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
		},
		{
			// Consumptions with improper timestamps
			name:                   "Test bad consumptions (Delta).",
			aggregationTemporality: cumulative,
			verifier:               verifyBadMetricsOkay,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
		},
	}

	for _, tc := range testcases {
		// Since parallelism is enabled in these tests, to avoid flaky behavior,
		// instantiate a copy of the test case for t.Run's closure to use.
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			mexp := &mocks.MetricsExporter{}
			tcon := &mocks.TracesConsumer{}

			// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
			mexp.On("ConsumeMetrics", mock.Anything, mock.MatchedBy(func(input pmetric.Metrics) bool {
				return assert.Eventually(t, func() bool {
					return tc.verifier(t, input)
				}, 10*time.Second, time.Millisecond*100)
			})).Return(nil)
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

			defaultNullValue := pcommon.NewValueStr("defaultNullValue")
			p := newProcessorImp(mexp, tcon, &defaultNullValue, tc.aggregationTemporality, zaptest.NewLogger(t), []ExcludePattern{})

			for _, traces := range tc.traces {
				// Test
				ctx := metadata.NewIncomingContext(context.Background(), nil)
				err := p.ConsumeTraces(ctx, traces)

				// Verify
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricKeyCache(t *testing.T) {
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, zaptest.NewLogger(t), []ExcludePattern{})
	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)

	// 0 key was cached at beginning
	assert.Zero(t, p.metricKeyToDimensions.Len())

	err := p.ConsumeTraces(ctx, traces)
	// Validate
	require.NoError(t, err)
	// 2 key was cached, 1 key was evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return assert.Equal(t, DimensionsCacheSize, p.metricKeyToDimensions.Len())
	}, 10*time.Second, time.Millisecond*100)

	// consume another batch of traces
	err = p.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// 2 key was cached, other keys were evicted and cleaned after the processing
	assert.Eventually(t, func() bool {
		return assert.Equal(t, DimensionsCacheSize, p.metricKeyToDimensions.Len())
	}, 10*time.Second, time.Millisecond*100)
}

func TestExcludePatternSkips(t *testing.T) {
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, observedLogger, []ExcludePattern{
		{
			Name:    "operation",
			Pattern: "p*",
		},
	})

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err := p.ConsumeTraces(ctx, traces)

	assert.NoError(t, err)
	found := false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Skipping span") {
			found = true
		}
	}
	assert.True(t, found)
}

func BenchmarkProcessorConsumeTraces(b *testing.B) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, cumulative, zaptest.NewLogger(b), []ExcludePattern{})

	traces := buildSampleTrace()

	// Test
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, p.ConsumeTraces(ctx, traces))
	}
}

func newProcessorImp(mexp *mocks.MetricsExporter, tcon *mocks.TracesConsumer, defaultNullValue *pcommon.Value, temporality string, logger *zap.Logger, excludePatterns []ExcludePattern) *processorImp {
	defaultNotInSpanAttrVal := pcommon.NewValueStr("defaultNotInSpanAttrVal")
	// use size 2 for LRU cache for testing purpose
	metricKeyToDimensions, _ := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)
	callMetricKeyToDimensions, _ := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)
	dbCallMetricKeyToDimensions, _ := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)
	externalCallMetricKeyToDimensions, _ := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)
	expHistogramKeyToDimensions, err := cache.NewCache[metricKey, pcommon.Map](DimensionsCacheSize)

	defaultDimensions := []dimension{
		// Set nil defaults to force a lookup for the attribute in the span.
		{stringAttrName, nil},
		{intAttrName, nil},
		{doubleAttrName, nil},
		{boolAttrName, nil},
		{mapAttrName, nil},
		{arrayAttrName, nil},
		{nullAttrName, defaultNullValue},
		// Add a default value for an attribute that doesn't exist in a span
		{notInSpanAttrName0, &defaultNotInSpanAttrVal},
		// Leave the default value unset to test that this dimension should not be added to the metric.
		{notInSpanAttrName1, nil},
		// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
		{regionResourceAttrName, nil},
	}

	defaultCallDimensions := append([]dimension{
		{
			name:  conventions.AttributeHTTPStatusCode,
			value: nil,
		},
	}, defaultDimensions...)
	dbCallDimensions := append([]dimension{
		{
			name:  conventions.AttributeDBSystem,
			value: nil,
		},
		{
			name:  conventions.AttributeDBName,
			value: nil,
		},
	}, defaultDimensions...)
	externalCallDimensions := append([]dimension{}, defaultDimensions...)

	if err != nil {
		panic(err)
	}

	excludePatternRegex := make(map[string]*regexp.Regexp)
	for _, pattern := range excludePatterns {
		excludePatternRegex[pattern.Name] = regexp.MustCompile(pattern.Pattern)
	}

	mexpArr := []consumer.Metrics{}
	mexpArr = append(mexpArr, mexp)
	return &processorImp{
		logger:          logger,
		config:          Config{AggregationTemporality: temporality},
		metricsConsumer: mexpArr,
		tracesConsumer:  tcon,

		startTimestamp:         pcommon.NewTimestampFromTime(time.Now()),
		histograms:             make(map[metricKey]*histogramData),
		expHistograms:          make(map[metricKey]*exponentialHistogram),
		callHistograms:         make(map[metricKey]*histogramData),
		dbCallHistograms:       make(map[metricKey]*histogramData),
		externalCallHistograms: make(map[metricKey]*histogramData),

		latencyBounds:             defaultLatencyHistogramBucketsMs,
		callLatencyBounds:         defaultLatencyHistogramBucketsMs,
		dbCallLatencyBounds:       defaultLatencyHistogramBucketsMs,
		externalCallLatencyBounds: defaultLatencyHistogramBucketsMs,

		dimensions: []dimension{
			// Set nil defaults to force a lookup for the attribute in the span.
			{stringAttrName, nil},
			{intAttrName, nil},
			{doubleAttrName, nil},
			{boolAttrName, nil},
			{mapAttrName, nil},
			{arrayAttrName, nil},
			{nullAttrName, defaultNullValue},
			// Add a default value for an attribute that doesn't exist in a span
			{notInSpanAttrName0, &defaultNotInSpanAttrVal},
			// Leave the default value unset to test that this dimension should not be added to the metric.
			{notInSpanAttrName1, nil},
			// Add a resource attribute to test "process" attributes like IP, host, region, cluster, etc.
			{regionResourceAttrName, nil},
		},
		callDimensions:         defaultCallDimensions,
		dbCallDimensions:       dbCallDimensions,
		externalCallDimensions: externalCallDimensions,

		keyBuf:                            new(bytes.Buffer),
		metricKeyToDimensions:             metricKeyToDimensions,
		expHistogramKeyToDimensions:       expHistogramKeyToDimensions,
		callMetricKeyToDimensions:         callMetricKeyToDimensions,
		dbCallMetricKeyToDimensions:       dbCallMetricKeyToDimensions,
		externalCallMetricKeyToDimensions: externalCallMetricKeyToDimensions,

		attrsCardinality:                       make(map[string]map[string]struct{}),
		serviceToOperations:                    make(map[string]map[string]struct{}),
		maxNumberOfServicesToTrack:             maxNumberOfServicesToTrack,
		maxNumberOfOperationsToTrackPerService: maxNumberOfOperationsToTrackPerService,
		excludePatternRegex:                    excludePatternRegex,
	}
}

// verifyConsumeMetricsInputCumulative expects one accumulation of metrics, and marked as cumulative
func verifyConsumeMetricsInputCumulative(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, 1)
}

func verifyBadMetricsOkay(t testing.TB, input pmetric.Metrics) bool {
	return true // Validating no exception
}

// verifyConsumeMetricsInputDelta expects one accumulation of metrics, and marked as delta
func verifyConsumeMetricsInputDelta(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityDelta, 1)
}

// verifyMultipleCumulativeConsumptions expects the amount of accumulations as kept track of by numCumulativeConsumptions.
// numCumulativeConsumptions acts as a multiplier for the values, since the cumulative metrics are additive.
func verifyMultipleCumulativeConsumptions() func(t testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(t testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

// verifyConsumeMetricsInput verifies the input of the ConsumeMetrics call from this processor.
// This is the best point to verify the computed metrics from spans are as expected.
func verifyConsumeMetricsInput(t testing.TB, input pmetric.Metrics, expectedTemporality pmetric.AggregationTemporality, numCumulativeConsumptions int) bool {
	require.Equal(t, 6, input.DataPointCount(),
		"Should be 3 for each of call count and latency. Each group of 3 data points is made of: "+
			"service-a (server kind) -> service-a (client kind) -> service-b (service kind)",
	)

	rm := input.ResourceMetrics()
	require.Equal(t, 1, rm.Len())

	ilm := rm.At(0).ScopeMetrics()
	require.Equal(t, 1, ilm.Len())
	assert.Equal(t, "signozspanmetricsprocessor", ilm.At(0).Scope().Name())

	m := ilm.At(0).Metrics()
	// 6 metrics: signoz_calls_total, signoz_latency, signoz_db_latency_sum
	// signoz_db_latency_count, signoz_external_latency_sum, signoz_external_latency_count
	require.Equal(t, 6, m.Len())

	seenMetricIDs := make(map[metricID]bool)
	// The first 3 data points are for call counts.
	assert.Equal(t, "signoz_calls_total", m.At(0).Name())
	assert.Equal(t, expectedTemporality, m.At(0).Sum().AggregationTemporality())
	assert.True(t, m.At(0).Sum().IsMonotonic())
	callsDps := m.At(0).Sum().DataPoints()
	require.Equal(t, 3, callsDps.Len())
	for dpi := 0; dpi < 3; dpi++ {
		dp := callsDps.At(dpi)
		assert.Equal(t, int64(numCumulativeConsumptions), dp.IntValue(), "There should only be one metric per Service/operation/kind combination")
		assert.NotZero(t, dp.StartTimestamp(), "StartTimestamp should be set")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")
		verifyMetricLabels(dp, t, seenMetricIDs)
	}

	seenMetricIDs = make(map[metricID]bool)
	// The remaining 3 data points are for latency.
	assert.Equal(t, "signoz_latency", m.At(1).Name())
	assert.Equal(t, "ms", m.At(1).Unit())
	assert.Equal(t, expectedTemporality, m.At(1).Histogram().AggregationTemporality())
	latencyDps := m.At(1).Histogram().DataPoints()
	require.Equal(t, 3, latencyDps.Len())
	for dpi := 0; dpi < 3; dpi++ {
		dp := latencyDps.At(dpi)
		assert.Equal(t, sampleLatency*float64(numCumulativeConsumptions), dp.Sum(), "Should be a 11ms latency measurement, multiplied by the number of stateful accumulations.")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")

		// Verify bucket counts.

		// The bucket counts should be 1 greater than the explicit bounds as documented in:
		// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto.
		assert.Equal(t, dp.ExplicitBounds().Len()+1, dp.BucketCounts().Len())

		// Find the bucket index where the 11ms latency should belong in.
		var foundLatencyIndex int
		for foundLatencyIndex = 0; foundLatencyIndex < dp.ExplicitBounds().Len(); foundLatencyIndex++ {
			if dp.ExplicitBounds().At(foundLatencyIndex) > sampleLatency {
				break
			}
		}

		// Then verify that all histogram buckets are empty except for the bucket with the 11ms latency.
		var wantBucketCount uint64
		for bi := 0; bi < dp.BucketCounts().Len(); bi++ {
			wantBucketCount = 0
			if bi == foundLatencyIndex {
				wantBucketCount = uint64(numCumulativeConsumptions)
			}
			assert.Equal(t, wantBucketCount, dp.BucketCounts().At(bi))
		}
		verifyMetricLabels(dp, t, seenMetricIDs)
	}
	return true
}

func verifyMetricLabels(dp metricDataPoint, t testing.TB, seenMetricIDs map[metricID]bool) {
	mID := metricID{}
	wantDimensions := map[string]pcommon.Value{
		conflictResourceAttr:                    pcommon.NewValueStr(sampleConflictingHost),
		resourcePrefix + conflictResourceAttr:   pcommon.NewValueStr(sampleConflictingHost),
		stringAttrName:                          pcommon.NewValueStr("stringAttrValue"),
		intAttrName:                             pcommon.NewValueInt(99),
		doubleAttrName:                          pcommon.NewValueDouble(99.99),
		boolAttrName:                            pcommon.NewValueBool(true),
		nullAttrName:                            pcommon.NewValueEmpty(),
		arrayAttrName:                           pcommon.NewValueSlice(),
		mapAttrName:                             pcommon.NewValueMap(),
		notInSpanAttrName0:                      pcommon.NewValueStr("defaultNotInSpanAttrVal"),
		regionResourceAttrName:                  pcommon.NewValueStr(sampleRegion),
		resourcePrefix + regionResourceAttrName: pcommon.NewValueStr(sampleRegion),
	}
	dp.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case serviceNameKey:
			mID.service = v.Str()
		case operationKey:
			mID.operation = v.Str()
		case spanKindKey:
			mID.kind = v.Str()
		case statusCodeKey:
			mID.statusCode = v.Str()
		case notInSpanAttrName1:
			assert.Fail(t, notInSpanAttrName1+" should not be in this metric")
		default:
			assert.Equal(t, wantDimensions[k], v)
			delete(wantDimensions, k)
		}
		return true
	})
	assert.Empty(t, wantDimensions, "Did not see all expected dimensions in metric. Missing: ", wantDimensions)

	// Service/operation/kind should be a unique metric.
	assert.False(t, seenMetricIDs[mID])
	seenMetricIDs[mID] = true
}

func buildBadSampleTrace() ptrace.Traces {
	badTrace := buildSampleTrace()
	span := badTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	now := time.Now()
	// Flipping timestamp for a bad duration
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))
	return badTrace
}

// buildSampleTrace builds the following trace:
//
//	service-a/ping (server) ->
//	  service-a/ping (client) ->
//	    service-b/ping (server)
func buildSampleTrace() ptrace.Traces {
	traces := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeOk,
				},
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindClient,
					statusCode: ptrace.StatusCodeOk,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					operation:  "/ping",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(serviceSpans{}, traces.ResourceSpans().AppendEmpty())
	return traces
}

func initServiceSpans(serviceSpans serviceSpans, spans ptrace.ResourceSpans) {
	if serviceSpans.serviceName != "" {
		spans.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceSpans.serviceName)
	}

	spans.Resource().Attributes().PutStr(regionResourceAttrName, sampleRegion)
	spans.Resource().Attributes().PutStr(conflictResourceAttr, sampleConflictingHost)

	ils := spans.ScopeSpans().AppendEmpty()
	for _, span := range serviceSpans.spans {
		initSpan(span, ils.Spans().AppendEmpty())
	}
}

func initSpan(span span, s ptrace.Span) {
	s.SetName(span.operation)
	s.SetKind(span.kind)
	s.Status().SetCode(span.statusCode)
	now := time.Now()
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))

	s.Attributes().PutStr(stringAttrName, "stringAttrValue")
	s.Attributes().PutStr(conflictResourceAttr, sampleConflictingHost)
	s.Attributes().PutStr("http.response.status_code", "200")
	s.Attributes().PutInt(intAttrName, 99)
	s.Attributes().PutDouble(doubleAttrName, 99.99)
	s.Attributes().PutBool(boolAttrName, true)
	s.Attributes().PutEmpty(nullAttrName)
	s.Attributes().PutEmptyMap(mapAttrName)
	s.Attributes().PutEmptySlice(arrayAttrName)
	s.SetTraceID(pcommon.TraceID([16]byte{byte(42)}))
	s.SetSpanID(pcommon.SpanID([8]byte{byte(42)}))
}

func newOTLPExporters(t *testing.T) (component.ID, exporter.Metrics, exporter.Traces) {
	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ClientConfig: configgrpc.ClientConfig{
			Endpoint: "example.com:1234",
		},
	}
	expCreationParams := exportertest.NewNopSettings(component.MustNewType("otlp"))
	mexp, err := otlpExpFactory.CreateMetrics(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	texp, err := otlpExpFactory.CreateTraces(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	otlpID := component.NewID(component.MustNewType("otlp"))
	return otlpID, mexp, texp
}

func TestBuildKeySameServiceOperationCharSequence(t *testing.T) {

	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, pmetric.AggregationTemporalityCumulative.String(), zaptest.NewLogger(t), []ExcludePattern{})

	span0 := ptrace.NewSpan()
	span0.SetName("c")
	buf := &bytes.Buffer{}
	p.buildKey(buf, "ab", span0, nil, pcommon.NewMap())
	k0 := metricKey(buf.String())
	buf.Reset()
	span1 := ptrace.NewSpan()
	span1.SetName("bc")
	p.buildKey(buf, "a", span1, nil, pcommon.NewMap())
	k1 := metricKey(buf.String())
	assert.NotEqual(t, k0, k1)
	assert.Equal(t, metricKey("ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k0)
	assert.Equal(t, metricKey("a\u0000bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k1)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, pmetric.AggregationTemporalityCumulative.String(), zaptest.NewLogger(t), []ExcludePattern{})

	defaultFoo := pcommon.NewValueStr("bar")
	for _, tc := range []struct {
		name            string
		optionalDims    []dimension
		resourceAttrMap map[string]interface{}
		spanAttrMap     map[string]interface{}
		wantKey         string
	}{
		{
			name:    "nil optionalDims",
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "neither span nor resource contains key, dim provides default",
			optionalDims: []dimension{
				{name: "foo", value: &defaultFoo},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "neither span nor resource contains key, dim provides no default",
			optionalDims: []dimension{
				{name: "foo"},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET",
		},
		{
			name: "span attribute contains dimension",
			optionalDims: []dimension{
				{name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute contains dimension",
			optionalDims: []dimension{
				{name: "foo"},
			},
			resourceAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "both span and resource attribute contains dimension, should prefer span attribute",
			optionalDims: []dimension{
				{name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 100,
			},
			resourceAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000100",
		},
		{
			name: "resource attribute contains instance ID",
			optionalDims: []dimension{
				{name: signozID},
			},
			resourceAttrMap: map[string]interface{}{
				signozID: testID,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000test-instance-id",
		},
		{
			name: "http status code with new sem conv",
			optionalDims: []dimension{
				{name: "http.response.status_code"},
			},
			spanAttrMap: map[string]interface{}{
				"http.response.status_code": 200,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000200",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resAttr := pcommon.NewMap()
			assert.NoError(t, resAttr.FromRaw(tc.resourceAttrMap))
			span0 := ptrace.NewSpan()
			assert.NoError(t, span0.Attributes().FromRaw(tc.spanAttrMap))
			span0.SetName("c")
			buf := &bytes.Buffer{}
			p.buildKey(buf, "ab", span0, tc.optionalDims, resAttr)
			assert.Equal(t, tc.wantKey, buf.String())
		})
	}
}

func TestProcessorDuplicateDimensions(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Duplicate dimension with reserved label after sanitization.
	cfg.Dimensions = []Dimension{
		{Name: "status_code"},
	}

	// Test
	p, err := newProcessor(zaptest.NewLogger(t), testID, cfg, nil)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestValidateDimensions(t *testing.T) {
	for _, tc := range []struct {
		name              string
		dimensions        []Dimension
		expectedErr       string
		skipSanitizeLabel bool
	}{
		{
			name:       "no additional dimensions",
			dimensions: []Dimension{},
		},
		{
			name: "no duplicate dimensions",
			dimensions: []Dimension{
				{Name: "http.service_name"},
				{Name: "http.status_code"},
			},
		},
		{
			name: "duplicate dimension with reserved labels",
			dimensions: []Dimension{
				{Name: "service.name"},
			},
			expectedErr: "duplicate dimension name service.name",
		},
		{
			name: "duplicate dimension with reserved labels after sanitization",
			dimensions: []Dimension{
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions",
			dimensions: []Dimension{
				{Name: "service_name"},
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions after sanitization",
			dimensions: []Dimension{
				{Name: "http.status_code"},
				{Name: "http!status_code"},
			},
			expectedErr: "duplicate dimension name http_status_code after sanitization",
		},
		{
			name: "we skip the case if the dimension name is the same after sanitization",
			dimensions: []Dimension{
				{Name: "http_status_code"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.skipSanitizeLabel = false
			err := validateDimensions(tc.dimensions, tc.skipSanitizeLabel)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Equal(t, "", sanitize("", cfg.skipSanitizeLabel), "")
	require.Equal(t, "key_test", sanitize("_test", cfg.skipSanitizeLabel))
	require.Equal(t, "key__test", sanitize("__test", cfg.skipSanitizeLabel))
	require.Equal(t, "key_0test", sanitize("0test", cfg.skipSanitizeLabel))
	require.Equal(t, "test", sanitize("test", cfg.skipSanitizeLabel))
	require.Equal(t, "test__", sanitize("test_/", cfg.skipSanitizeLabel))
	// testcases with skipSanitizeLabel flag turned on
	cfg.skipSanitizeLabel = true
	require.Equal(t, "", sanitize("", cfg.skipSanitizeLabel), "")
	require.Equal(t, "_test", sanitize("_test", cfg.skipSanitizeLabel))
	require.Equal(t, "key__test", sanitize("__test", cfg.skipSanitizeLabel))
	require.Equal(t, "key_0test", sanitize("0test", cfg.skipSanitizeLabel))
	require.Equal(t, "test", sanitize("test", cfg.skipSanitizeLabel))
	require.Equal(t, "test__", sanitize("test_/", cfg.skipSanitizeLabel))
}

func TestSetExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	spanID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SpanID()
	exemplarSlice := pmetric.NewExemplarSlice()
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	value := float64(42)

	ed := []exemplarData{{traceID: traceID, spanID: spanID, value: value}}

	// ----- call -------------------------------------------------------------
	setExemplars(ed, timestamp, exemplarSlice)

	// ----- verify -----------------------------------------------------------
	traceIDValue := exemplarSlice.At(0).TraceID()
	spanIDValue := exemplarSlice.At(0).SpanID()

	assert.NotEmpty(t, exemplarSlice)
	assert.Equal(t, traceIDValue, traceID)
	assert.Equal(t, spanIDValue, spanID)
	assert.Equal(t, exemplarSlice.At(0).Timestamp(), timestamp)
	assert.Equal(t, exemplarSlice.At(0).DoubleValue(), value)
}

func TestProcessorUpdateExemplars(t *testing.T) {
	// ----- conditions -------------------------------------------------------
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	traces := buildSampleTrace()
	traceID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
	spanID := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SpanID()
	key := metricKey("metricKey")
	next := new(consumertest.TracesSink)
	p, err := newProcessor(zaptest.NewLogger(t), testID, cfg, nil)
	p.tracesConsumer = next
	value := float64(42)

	// ----- call -------------------------------------------------------------
	p.updateHistogram(key, value, traceID, spanID)

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.NotEmpty(t, p.histograms[key].exemplarsData)
	assert.Equal(t, p.histograms[key].exemplarsData[0], exemplarData{traceID: traceID, spanID: spanID, value: value})

	// ----- call -------------------------------------------------------------
	p.resetExemplarData()

	// ----- verify -----------------------------------------------------------
	assert.NoError(t, err)
	assert.Empty(t, p.histograms[key].exemplarsData)
}

func TestBuildKeyWithDimensionsOverflow(t *testing.T) {
	// Prepare
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}

	// Mocked metric exporter will perform validation on metrics, during p.ConsumeTraces()
	mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
	tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(nil)

	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)

	defaultNullValue := pcommon.NewValueStr("defaultNullValue")
	p := newProcessorImp(mexp, tcon, &defaultNullValue, pmetric.AggregationTemporalityCumulative.String(), observedLogger, []ExcludePattern{})
	resAttr := pcommon.NewMap()

	for i := 0; i <= p.maxNumberOfServicesToTrack; i++ {
		span0 := ptrace.NewSpan()
		span0.SetName("span")
		buf := &bytes.Buffer{}
		serviceName := fmt.Sprintf("service-%d", i)
		p.buildKey(buf, serviceName, span0, []dimension{}, resAttr)
	}

	// adding new service should result in overflow
	span0 := ptrace.NewSpan()
	span0.SetName("span")
	buf := &bytes.Buffer{}
	serviceName := fmt.Sprintf("service-%d", p.maxNumberOfServicesToTrack)
	p.buildKey(buf, serviceName, span0, []dimension{}, resAttr)

	assert.Contains(t, buf.String(), overflowServiceName)

	found := false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Too many services to track, using overflow service name") {
			found = true
		}
	}
	assert.True(t, found)

	// reset to test operations
	p.serviceToOperations = make(map[string]map[string]struct{})
	p.buildKey(buf, "simple_service", span0, []dimension{}, resAttr)

	for i := 0; i <= p.maxNumberOfOperationsToTrackPerService; i++ {
		span0 := ptrace.NewSpan()
		span0.SetName(fmt.Sprintf("operation-%d", i))
		buf := &bytes.Buffer{}
		p.buildKey(buf, "simple_service", span0, []dimension{}, resAttr)
	}

	// adding a new operation to service "simple_service" should result in overflow
	span0 = ptrace.NewSpan()
	span0.SetName(fmt.Sprintf("operation-%d", p.maxNumberOfOperationsToTrackPerService))
	buf = &bytes.Buffer{}
	p.buildKey(buf, "simple_service", span0, []dimension{}, resAttr)
	assert.Contains(t, buf.String(), overflowOperation)

	found = false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Too many operations to track, using overflow operation name") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestParseTimesFromKeyOrNow(t *testing.T) {
	interval := time.Minute

	// Fixed times to avoid flakiness
	validSpanStart := time.Date(2024, 1, 1, 12, 30, 15, 0, time.UTC)
	validBucket := validSpanStart.Truncate(interval)
	now := time.Date(2024, 1, 1, 12, 30, 30, 0, time.UTC)
	processorStart := pcommon.NewTimestampFromTime(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC))

	// Keys
	prefixedKey := metricKey(fmt.Sprintf("%d%s%s",
		validBucket.Unix(),
		metricKeySeparator,
		"test-service\x00test-operation\x00SPAN_KIND_SERVER\x00STATUS_CODE_OK",
	))
	legacyKey := metricKey("test-service\x00test-operation\x00SPAN_KIND_SERVER\x00STATUS_CODE_OK")
	malformedKey := metricKey("invalid\x00test-service\x00test-operation")

	tests := []struct {
		name      string
		key       metricKey
		now       time.Time
		procStart pcommon.Timestamp
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "Prefixed_Valid",
			key:       prefixedKey,
			now:       now,
			procStart: processorStart,
			wantStart: validBucket,
			wantEnd:   validBucket, // For delta: both start and end are bucket start
		},
		{
			name:      "Legacy_NoPrefix",
			key:       legacyKey,
			now:       now,
			procStart: processorStart,
			wantStart: processorStart.AsTime(),
			wantEnd:   now,
		},
		{
			name:      "Malformed_Prefix",
			key:       malformedKey,
			now:       now,
			procStart: processorStart,
			wantStart: processorStart.AsTime(),
			wantEnd:   now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := parseTimesFromKeyOrNow(tc.key, interval, tc.now, tc.procStart)
			assert.Equal(t, pcommon.NewTimestampFromTime(tc.wantStart), start)
			assert.Equal(t, pcommon.NewTimestampFromTime(tc.wantEnd), end)
		})
	}
}

func TestBuildMetricKeyConditionalTimeBucketing(t *testing.T) {
	// testing example time
	startTime := time.Date(2024, 1, 1, 12, 30, 15, 0, time.UTC)
	timeBucketInterval := time.Minute
	// testing example span
	span := ptrace.NewSpan()
	span.SetName("test-operation")
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(100 * time.Millisecond)))

	resourceAttr := pcommon.NewMap()
	serviceName := "test-service"

	// Test Delta Temporality - should include time bucket prefix
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}
	logger := zap.NewNop()

	// initialize delta processor
	deltaProcessor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_DELTA", logger, nil)
	deltaProcessor.config.TimeBucketInterval = timeBucketInterval

	// function call to get a key from the function
	deltaKey := deltaProcessor.buildMetricKey(serviceName, span, nil, resourceAttr)

	// calculate the expected bucket timestamp
	expectedBucket := startTime.Truncate(timeBucketInterval).Unix()

	// verify the key starts with the bucket timestamp for delta temporality
	assert.True(t, strings.HasPrefix(string(deltaKey), strconv.FormatInt(expectedBucket, 10)))

	// initialize cumulative processor
	cumulativeProcessor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_CUMULATIVE", logger, nil)
	cumulativeProcessor.config.TimeBucketInterval = timeBucketInterval

	// function call to get a key from the function
	cumulativeKey := cumulativeProcessor.buildMetricKey(serviceName, span, nil, resourceAttr)

	// verify the key does NOT start with the bucket timestamp for cumulative temporality
	assert.False(t, strings.HasPrefix(string(cumulativeKey), strconv.FormatInt(expectedBucket, 10)))

	// verify the cumulative key starts with the service name
	assert.True(t, strings.HasPrefix(string(cumulativeKey), serviceName))
}

func TestBuildCustomMetricKeyConditionalTimeBucketing(t *testing.T) {
	// testing example time
	startTime := time.Date(2024, 1, 1, 12, 30, 15, 0, time.UTC)
	timeBucketInterval := time.Minute
	// testing example span
	span := ptrace.NewSpan()
	span.SetName("test-operation")
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(100 * time.Millisecond)))

	resourceAttr := pcommon.NewMap()
	serviceName := "test-service"

	extraVals := []string{"example.com:8080"}

	// initialize delta processor
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}
	logger := zap.NewNop()

	deltaProcessor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_DELTA", logger, nil)
	deltaProcessor.config.TimeBucketInterval = timeBucketInterval

	// function call to get a key from the function
	deltaKey := deltaProcessor.buildCustomMetricKey(serviceName, span, nil, resourceAttr, extraVals)

	// calculate the expected bucket
	expectedBucket := startTime.Truncate(timeBucketInterval).Unix()

	// verify the key starts with the bucket timestamp for delta temporality
	assert.True(t, strings.HasPrefix(string(deltaKey), strconv.FormatInt(expectedBucket, 10)))

	// initialize cumulative processor
	cumulativeProcessor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_CUMULATIVE", logger, nil)
	cumulativeProcessor.config.TimeBucketInterval = timeBucketInterval

	// function call to get a key from the function
	cumulativeKey := cumulativeProcessor.buildCustomMetricKey(serviceName, span, nil, resourceAttr, extraVals)

	// verify the key does NOT start with the bucket timestamp for cumulative temporality
	assert.False(t, strings.HasPrefix(string(cumulativeKey), strconv.FormatInt(expectedBucket, 10)))

	// The cumulative key should start with the service name
	assert.True(t, strings.HasPrefix(string(cumulativeKey), serviceName))
}

func TestBuildMetricsTimestampAccuracy(t *testing.T) {
	// Tests that buildMetrics() generates metrics with correct timestamps based on temporality mode.
	// Verifies that:
	// 1. DELTA temporality: metrics use bucket timestamps derived from span start times (timestamp-aware)
	// 2. CUMULATIVE temporality: metrics use processor start time + current time (original behavior)
	// 3. Timestamp values appear in final metric data points, not just during aggregation
	// This validates the end-to-end timestamp accuracy for dashboard correlation and late-arriving spans

	// Define time buckets for predictable testing
	bucket1Start := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC) // 12:30:00
	bucket1End := bucket1Start.Add(time.Minute)                   // 12:31:00

	t.Run("Delta_Uses_Bucket_Timestamps", func(t *testing.T) {
		// Use existing helper function to create properly initialized processor
		mexp := &mocks.MetricsExporter{}
		tcon := &mocks.TracesConsumer{}
		logger := zap.NewNop()

		// Create processor with delta temporality
		processor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_DELTA", logger, nil)
		// Prevent staleness guard from skipping this backdated test span (2024)
		processor.config.SkipSpansOlderThan = 100 * 365 * 24 * time.Hour // setting for 100 years

		// Configure time bucketing
		processor.config.TimeBucketInterval = time.Minute

		// Create resource attributes
		resourceAttr := pcommon.NewMap()
		resourceAttr.PutStr("service.name", "test-service")
		serviceName := "test-service"

		// Create span with specific start time that will fall into bucket1
		span := ptrace.NewSpan()
		span.SetName("test-span")
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(bucket1Start.Add(15 * time.Second))) // 12:30:15
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(bucket1Start.Add(45 * time.Second)))   // 12:30:45

		// Aggregate metrics for this span
		processor.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		// Build metrics
		metrics, err := processor.buildMetrics()
		assert.NoError(t, err, "buildMetrics should not return error")

		// Verify metrics structure
		require.Equal(t, 1, metrics.ResourceMetrics().Len())
		ilm := metrics.ResourceMetrics().At(0).ScopeMetrics()
		require.Equal(t, 1, ilm.Len())
		assert.Equal(t, "signozspanmetricsprocessor", ilm.At(0).Scope().Name())

		// Get all metrics
		allMetrics := ilm.At(0).Metrics()
		require.Greater(t, allMetrics.Len(), 0, "Should have at least one metric")

		// Verify timestamp accuracy for each metric
		foundDeltaTimestamps := false
		for i := 0; i < allMetrics.Len(); i++ {
			metric := allMetrics.At(i)

			switch metric.Type() {
			case pmetric.MetricTypeHistogram:
				histogram := metric.Histogram()
				dps := histogram.DataPoints()

				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)

					serviceNameAttr, hasService := dp.Attributes().Get("service.name")
					require.True(t, hasService, "Data point should have service.name attribute")

					if serviceNameAttr.Str() == serviceName {
						// For delta temporality, timestamps should match bucket boundaries
						startTime := dp.StartTimestamp().AsTime()
						timestamp := dp.Timestamp().AsTime()

						// Verify timestamps match expected bucket boundaries
						assert.Equal(t, bucket1Start, startTime,
							"StartTimestamp should match bucket start time for delta temporality")
						assert.Equal(t, bucket1Start, timestamp,
							"Timestamp should match bucket start time for delta temporality")

						// Verify timestamps are NOT close to current time
						now := time.Now()
						assert.False(t, timestamp.After(now.Add(-5*time.Second)) && timestamp.Before(now.Add(5*time.Second)),
							"Timestamp should NOT be close to current time for delta temporality")

						// Verify data integrity
						assert.Greater(t, dp.Count(), uint64(0), "Histogram should have count > 0")
						assert.Greater(t, dp.Sum(), float64(0), "Histogram should have sum > 0")

						foundDeltaTimestamps = true
						t.Logf("Delta histogram - StartTime: %v, EndTime: %v", startTime, timestamp)
					}
				}

			case pmetric.MetricTypeSum:
				sum := metric.Sum()
				dps := sum.DataPoints()

				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)

					serviceNameAttr, hasService := dp.Attributes().Get("service.name")
					require.True(t, hasService, "Data point should have service.name attribute")

					if serviceNameAttr.Str() == serviceName {
						// For delta temporality, timestamps should match bucket boundaries
						startTime := dp.StartTimestamp().AsTime()
						timestamp := dp.Timestamp().AsTime()

						// Verify timestamps match expected bucket boundaries
						assert.Equal(t, bucket1Start, startTime,
							"StartTimestamp should match bucket start time for delta temporality")
						assert.Equal(t, bucket1Start, timestamp,
							"Timestamp should match bucket start time for delta temporality")

						// Verify data integrity
						assert.Greater(t, dp.IntValue(), int64(0), "Sum should have value > 0")

						foundDeltaTimestamps = true
						t.Logf("Delta sum - StartTime: %v, EndTime: %v", startTime, timestamp)
					}
				}
			}
		}

		assert.True(t, foundDeltaTimestamps, "Should have found at least one data point with delta timestamps")
	})

	t.Run("Cumulative_Uses_Current_Time", func(t *testing.T) {
		// Use existing helper function to create properly initialized processor
		mexp := &mocks.MetricsExporter{}
		tcon := &mocks.TracesConsumer{}
		logger := zap.NewNop()

		// Create processor with cumulative temporality
		processor := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_CUMULATIVE", logger, nil)
		// Prevent staleness guard from skipping this backdated test span (2024)
		processor.config.SkipSpansOlderThan = 100 * 365 * 24 * time.Hour // setting for 100 years

		// Configure time bucketing (but it shouldn't be used for cumulative)
		processor.config.TimeBucketInterval = time.Minute

		// Create resource attributes
		resourceAttr := pcommon.NewMap()
		resourceAttr.PutStr("service.name", "test-service")
		serviceName := "test-service"

		// Create span with specific start time
		span := ptrace.NewSpan()
		span.SetName("cumulative-span")
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(bucket1Start.Add(15 * time.Second))) // 12:30:15
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(bucket1Start.Add(45 * time.Second)))   // 12:30:45

		// Aggregate metrics for this span
		processor.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		// Record time before building metrics
		beforeBuild := time.Now()

		// Build metrics
		metrics, err := processor.buildMetrics()
		assert.NoError(t, err, "buildMetrics should not return error")

		// Verify metrics structure
		require.Equal(t, 1, metrics.ResourceMetrics().Len())
		ilm := metrics.ResourceMetrics().At(0).ScopeMetrics()
		require.Equal(t, 1, ilm.Len())

		// Get all metrics
		allMetrics := ilm.At(0).Metrics()
		require.Greater(t, allMetrics.Len(), 0, "Should have at least one metric")

		// Verify timestamp accuracy for each metric
		foundCumulativeTimestamps := false
		for i := 0; i < allMetrics.Len(); i++ {
			metric := allMetrics.At(i)

			switch metric.Type() {
			case pmetric.MetricTypeHistogram:
				histogram := metric.Histogram()
				dps := histogram.DataPoints()

				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)

					serviceNameAttr, hasService := dp.Attributes().Get("service.name")
					require.True(t, hasService, "Data point should have service.name attribute")

					if serviceNameAttr.Str() == serviceName {
						// For cumulative temporality, timestamps should use processor start time and current time
						// This is the correct behavior that matches the original main branch
						startTime := dp.StartTimestamp().AsTime()
						timestamp := dp.Timestamp().AsTime()

						processorStartTime := processor.startTimestamp.AsTime()

						// StartTimestamp should be the processor start time
						assert.Equal(t, processorStartTime, startTime,
							"StartTimestamp should be processor start time for cumulative temporality")

						// Timestamp should be close to current time (within a few seconds of beforeBuild)
						assert.True(t, timestamp.After(beforeBuild.Add(-5*time.Second)) && timestamp.Before(beforeBuild.Add(5*time.Second)),
							"Timestamp should be close to current time for cumulative temporality")

						// Verify timestamps are NOT the span bucket times (from 2024)
						assert.False(t, startTime.Equal(bucket1Start),
							"StartTimestamp should NOT match span bucket start time for cumulative temporality")
						assert.False(t, timestamp.Equal(bucket1End),
							"Timestamp should NOT match span bucket end time for cumulative temporality")

						// Verify data integrity
						assert.Greater(t, dp.Count(), uint64(0), "Histogram should have count > 0")
						assert.Greater(t, dp.Sum(), float64(0), "Histogram should have sum > 0")

						foundCumulativeTimestamps = true
						t.Logf("Cumulative histogram - StartTime: %v, EndTime: %v", startTime, timestamp)
					}
				}

			case pmetric.MetricTypeSum:
				sum := metric.Sum()
				dps := sum.DataPoints()

				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)

					serviceNameAttr, hasService := dp.Attributes().Get("service.name")
					require.True(t, hasService, "Data point should have service.name attribute")

					if serviceNameAttr.Str() == serviceName {
						// For cumulative temporality, timestamps should use processor start time and current time
						// This is the correct behavior that matches the original main branch
						startTime := dp.StartTimestamp().AsTime()
						timestamp := dp.Timestamp().AsTime()

						processorStartTime := processor.startTimestamp.AsTime()

						// StartTimestamp should be the processor start time
						assert.Equal(t, processorStartTime, startTime,
							"StartTimestamp should be processor start time for cumulative temporality")

						// Timestamp should be close to current time (within a few seconds of beforeBuild)
						assert.True(t, timestamp.After(beforeBuild.Add(-5*time.Second)) && timestamp.Before(beforeBuild.Add(5*time.Second)),
							"Timestamp should be close to current time for cumulative temporality")

						// Verify timestamps are NOT the span bucket times (from 2024)
						assert.False(t, startTime.Equal(bucket1Start),
							"StartTimestamp should NOT match span bucket start time for cumulative temporality")
						assert.False(t, timestamp.Equal(bucket1End),
							"Timestamp should NOT match span bucket end time for cumulative temporality")

						// Verify data integrity
						assert.Greater(t, dp.IntValue(), int64(0), "Sum should have value > 0")

						foundCumulativeTimestamps = true
						t.Logf("Cumulative sum - StartTime: %v, EndTime: %v", startTime, timestamp)
					}
				}
			}
		}

		assert.True(t, foundCumulativeTimestamps, "Should have found at least one data point with cumulative timestamps")
	})
}

// Tests the skip_spans_older_than staleness guard: spans older than the configured
// window are skipped before any aggregation; boundary and recent spans are accepted.
func TestSkipSpansOlderThan(t *testing.T) {
	// Common setup
	mexp := &mocks.MetricsExporter{}
	tcon := &mocks.TracesConsumer{}
	logger := zap.NewNop()

	resourceAttr := pcommon.NewMap()
	resourceAttr.PutStr("service.name", "svc")
	serviceName := "svc"

	window := 24 * time.Hour
	now := time.Now()

	t.Run("Delta_Skips_Stale_Spans", func(t *testing.T) {
		p := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_DELTA", logger, nil)
		p.config.SkipSpansOlderThan = window

		span := ptrace.NewSpan()
		span.SetName("stale-delta")
		span.SetKind(ptrace.SpanKindServer)
		staleStart := now.Add(-window - time.Hour)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(staleStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(staleStart.Add(10 * time.Millisecond)))

		p.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		// Nothing should be aggregated (since the span is stale)
		require.Equal(t, 0, len(p.histograms))
		require.Equal(t, 0, len(p.callHistograms))
		require.Equal(t, 0, len(p.dbCallHistograms))
		require.Equal(t, 0, len(p.externalCallHistograms))
	})

	t.Run("Delta_Accepts_Recent_Span", func(t *testing.T) {
		p := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_DELTA", logger, nil)
		p.config.SkipSpansOlderThan = window

		span := ptrace.NewSpan()
		span.SetName("boundary-delta")
		span.SetKind(ptrace.SpanKindServer)
		boundaryStart := now.Add(-window).Add(1 * time.Millisecond) // slightly inside window to avoid flakiness
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(boundaryStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(boundaryStart.Add(10 * time.Millisecond)))

		p.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		// At least one histogram should be created
		require.NotEqual(t, 0, len(p.histograms))
		require.NotEqual(t, 0, len(p.callHistograms))
	})

	t.Run("Cumulative_Skips_Stale_Spans", func(t *testing.T) {
		p := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_CUMULATIVE", logger, nil)
		p.config.SkipSpansOlderThan = window

		span := ptrace.NewSpan()
		span.SetName("stale-cumulative")
		span.SetKind(ptrace.SpanKindServer)
		staleStart := now.Add(-window - 2*time.Hour)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(staleStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(staleStart.Add(5 * time.Millisecond)))

		p.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		require.Equal(t, 0, len(p.histograms))
		require.Equal(t, 0, len(p.callHistograms))
	})

	// For cumulative temporality, boundary spans should be accepted just like delta
	t.Run("Cumulative_Accepts_Boundary_Span", func(t *testing.T) {
		p := newProcessorImp(mexp, tcon, nil, "AGGREGATION_TEMPORALITY_CUMULATIVE", logger, nil)
		p.config.SkipSpansOlderThan = window

		span := ptrace.NewSpan()
		span.SetName("boundary-cumulative")
		span.SetKind(ptrace.SpanKindServer)
		boundaryStart := now.Add(-window).Add(1 * time.Millisecond)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(boundaryStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(boundaryStart.Add(10 * time.Millisecond)))

		p.aggregateMetricsForSpan(serviceName, span, resourceAttr)

		require.NotEqual(t, 0, len(p.histograms))
		require.NotEqual(t, 0, len(p.callHistograms))
	})
}
