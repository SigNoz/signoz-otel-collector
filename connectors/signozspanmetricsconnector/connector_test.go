package signozspanmetricsconnector

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
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

func buildBadSampleTrace() ptrace.Traces {
	badTrace := buildSampleTrace()
	span := badTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	now := time.Now()
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))
	return badTrace
}

func verifyConsumeMetricsInputCumulative(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, 1)
}

func verifyBadMetricsOkay(t testing.TB, input pmetric.Metrics) bool {
	return true
}

func verifyConsumeMetricsInputDelta(t testing.TB, input pmetric.Metrics) bool {
	return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityDelta, 1)
}

func verifyMultipleCumulativeConsumptions() func(t testing.TB, input pmetric.Metrics) bool {
	numCumulativeConsumptions := 0
	return func(t testing.TB, input pmetric.Metrics) bool {
		numCumulativeConsumptions++
		return verifyConsumeMetricsInput(t, input, pmetric.AggregationTemporalityCumulative, numCumulativeConsumptions)
	}
}

func verifyConsumeMetricsInput(
	t testing.TB,
	input pmetric.Metrics,
	expectedTemporality pmetric.AggregationTemporality,
	numCumulativeConsumptions int,
) bool {
	t.Helper()
	require.Contains(t, []int{6, 7}, input.DataPointCount(),
		"Should be 3 for each of call count and latency. Each group of 3 data points is made of: "+
			"service-a (server kind) -> service-a (client kind) -> service-b (service kind)",
	)

	rm := input.ResourceMetrics()
	require.Equal(t, 1, rm.Len())

	ilm := rm.At(0).ScopeMetrics()
	require.Equal(t, 1, ilm.Len())
	assert.Equal(t, "signozspanmetricsprocessor", ilm.At(0).Scope().Name())

	m := ilm.At(0).Metrics()
	require.True(t, m.Len() == 6 || m.Len() == 7, "unexpected number of metrics: %d", m.Len())

	seenMetricIDs := make(map[metricID]bool)
	assert.Equal(t, "signoz_calls_total", m.At(0).Name())
	assert.Equal(t, expectedTemporality, m.At(0).Sum().AggregationTemporality())
	assert.True(t, m.At(0).Sum().IsMonotonic())
	callsDps := m.At(0).Sum().DataPoints()
	require.Equal(t, 3, callsDps.Len())
	for dpi := 0; dpi < callsDps.Len(); dpi++ {
		dp := callsDps.At(dpi)
		assert.Equal(t, int64(numCumulativeConsumptions), dp.IntValue())
		assert.NotZero(t, dp.StartTimestamp(), "StartTimestamp should be set")
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")
		verifyMetricLabels(dp, t, seenMetricIDs)
	}

	seenMetricIDs = make(map[metricID]bool)
	assert.Equal(t, "signoz_latency", m.At(1).Name())
	assert.Equal(t, "ms", m.At(1).Unit())
	assert.Equal(t, expectedTemporality, m.At(1).Histogram().AggregationTemporality())
	latencyDps := m.At(1).Histogram().DataPoints()
	require.Equal(t, 3, latencyDps.Len())
	for dpi := 0; dpi < latencyDps.Len(); dpi++ {
		dp := latencyDps.At(dpi)
		assert.Equal(t, sampleLatency*float64(numCumulativeConsumptions), dp.Sum())
		assert.NotZero(t, dp.Timestamp(), "Timestamp should be set")
		assert.Equal(t, dp.ExplicitBounds().Len()+1, dp.BucketCounts().Len())

		var foundLatencyIndex int
		for foundLatencyIndex = 0; foundLatencyIndex < dp.ExplicitBounds().Len(); foundLatencyIndex++ {
			if dp.ExplicitBounds().At(foundLatencyIndex) > sampleLatency {
				break
			}
		}

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

	assert.Equal(t, "signoz_external_call_latency_sum", m.At(2).Name())
	assert.Equal(t, "signoz_external_call_latency_count", m.At(3).Name())
	assert.Equal(t, "signoz_db_latency_sum", m.At(4).Name())
	assert.Equal(t, "signoz_db_latency_count", m.At(5).Name())
	if m.Len() == 7 {
		assert.Equal(t, "signoz_latency", m.At(6).Name())
		assert.Equal(t, pmetric.MetricTypeExponentialHistogram, m.At(6).Type())
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
		case "http.status_code":
			assert.Equal(t, "200", v.Str())
		case notInSpanAttrName1:
			assert.Fail(t, notInSpanAttrName1+" should not be in this metric")
		default:
			assert.Equal(t, wantDimensions[k], v)
			delete(wantDimensions, k)
		}
		return true
	})
	assert.Empty(t, wantDimensions, "Did not see all expected dimensions in metric. Missing: ", wantDimensions)

	assert.False(t, seenMetricIDs[mID])
	seenMetricIDs[mID] = true
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

const (
	defaultNullValue        = "defaultNullValue"
	defaultNotInSpanAttrVal = "defaultNotInSpanAttrVal"
)

func TestConnectorStart(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.MetricsFlushInterval = 10 * time.Millisecond

	fakeClock := clockwork.NewFakeClock()
	conn, sink := newTestConnectorInstance(t, cfg, fakeClock, zaptest.NewLogger(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	assert.True(t, conn.started)

	require.NoError(t, conn.ConsumeTraces(ctx, buildSampleTrace()))
	fakeClock.Advance(cfg.MetricsFlushInterval)

	assert.Eventually(t, func() bool { return len(sink.AllMetrics()) > 0 }, time.Second, 10*time.Millisecond)

	require.NoError(t, conn.Shutdown(ctx))
	assert.False(t, conn.started)
}

func TestConnectorShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	configureTestDimensions(cfg)

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))
	err := conn.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConfigureLatencyBounds(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.LatencyHistogramBuckets = []time.Duration{
		3 * time.Nanosecond,
		3 * time.Microsecond,
		3 * time.Millisecond,
		3 * time.Second,
	}

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))
	assert.Equal(t, []float64{0.000003, 0.003, 3, 3000}, conn.latencyBounds)
}

func TestConnectorCapabilities(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	configureTestDimensions(cfg)

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))
	caps := conn.Capabilities()
	assert.False(t, caps.MutatesData)
}

func TestConnectorConsumeTraces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name                   string
		aggregationTemporality string
		traces                 []ptrace.Traces
		verifier               func(t testing.TB, input pmetric.Metrics) bool
	}{
		{
			name:                   "single consumption cumulative",
			aggregationTemporality: cumulative,
			traces:                 []ptrace.Traces{buildSampleTrace()},
			verifier:               verifyConsumeMetricsInputCumulative,
		},
		{
			name:                   "single consumption delta",
			aggregationTemporality: delta,
			traces:                 []ptrace.Traces{buildSampleTrace()},
			verifier:               verifyConsumeMetricsInputDelta,
		},
		{
			name:                   "multiple cumulative consumptions",
			aggregationTemporality: cumulative,
			traces:                 []ptrace.Traces{buildSampleTrace(), buildSampleTrace()},
			verifier:               verifyMultipleCumulativeConsumptions(),
		},
		{
			name:                   "bad timestamps cumulative",
			aggregationTemporality: cumulative,
			traces:                 []ptrace.Traces{buildBadSampleTrace()},
			verifier:               verifyBadMetricsOkay,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			configureTestDimensions(cfg)
			cfg.AggregationTemporality = tc.aggregationTemporality
			cfg.DimensionsCacheSize = 1000

			conn, sink := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))

			ctx := context.Background()
			for _, tr := range tc.traces {
				require.NoError(t, conn.ConsumeTraces(ctx, tr))
				conn.exportMetrics(ctx)
			}

			got := sink.AllMetrics()
			require.Len(t, got, len(tc.traces))

			for _, metrics := range got {
				assert.True(t, tc.verifier(t, metrics))
			}
		})
	}
}

func TestMetricKeyCache(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.AggregationTemporality = cumulative
	cfg.DimensionsCacheSize = DimensionsCacheSize

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))

	ctx := context.Background()
	traces := buildSampleTrace()

	assert.Zero(t, conn.metricKeyToDimensions.Len())

	require.NoError(t, conn.ConsumeTraces(ctx, traces))
	conn.exportMetrics(ctx)
	assert.Equal(t, DimensionsCacheSize, conn.metricKeyToDimensions.Len())

	require.NoError(t, conn.ConsumeTraces(ctx, traces))
	conn.exportMetrics(ctx)
	assert.Equal(t, DimensionsCacheSize, conn.metricKeyToDimensions.Len())
}

func TestExcludePatternSkips(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.ExcludePatterns = []ExcludePattern{
		{
			Name:    "operation",
			Pattern: "p*",
		},
	}

	observedCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedCore)

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), logger)

	ctx := context.Background()
	require.NoError(t, conn.ConsumeTraces(ctx, buildSampleTrace()))
	conn.exportMetrics(ctx)

	found := false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Skipping span") {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestBuildKeySameServiceOperationCharSequence(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))

	span0 := ptrace.NewSpan()
	span0.SetName("c")
	buf := &bytes.Buffer{}
	conn.buildKey(buf, "ab", span0, nil, pcommon.NewMap())
	k0 := metricKey(buf.String())
	buf.Reset()

	span1 := ptrace.NewSpan()
	span1.SetName("bc")
	conn.buildKey(buf, "a", span1, nil, pcommon.NewMap())
	k1 := metricKey(buf.String())

	assert.NotEqual(t, k0, k1)
	assert.Equal(t, metricKey("ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k0)
	assert.Equal(t, metricKey("a\u0000bc\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET"), k1)
}

func TestBuildKeyWithDimensions(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))

	defaultFoo := pcommon.NewValueStr("bar")
	testCases := []struct {
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
			name: "dimension defaulted",
			optionalDims: []dimension{
				{name: "foo", value: &defaultFoo},
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000bar",
		},
		{
			name: "span attribute overrides",
			optionalDims: []dimension{
				{name: "foo"},
			},
			spanAttrMap: map[string]interface{}{
				"foo": 99,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u000099",
		},
		{
			name: "resource attribute used",
			optionalDims: []dimension{
				{name: "foo"},
			},
			resourceAttrMap: map[string]interface{}{
				"foo": 100,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000100",
		},
		{
			name: "span preferred over resource",
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
			name: "http status code stable attribute",
			optionalDims: []dimension{
				{name: "http.response.status_code"},
			},
			spanAttrMap: map[string]interface{}{
				"http.response.status_code": 200,
			},
			wantKey: "ab\u0000c\u0000SPAN_KIND_UNSPECIFIED\u0000STATUS_CODE_UNSET\u0000200",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resAttr := pcommon.NewMap()
			assert.NoError(t, resAttr.FromRaw(tc.resourceAttrMap))
			span0 := ptrace.NewSpan()
			assert.NoError(t, span0.Attributes().FromRaw(tc.spanAttrMap))
			span0.SetName("c")
			buf := &bytes.Buffer{}
			conn.buildKey(buf, "ab", span0, tc.optionalDims, resAttr)
			assert.Equal(t, tc.wantKey, buf.String())
		})
	}
}

func TestConnectorDuplicateDimensions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Dimensions = []Dimension{
		{Name: "status_code"},
	}

	_, err := newConnector(zaptest.NewLogger(t), cfg, clockwork.NewRealClock(), testID)
	assert.Error(t, err)
}

func TestConnectorUpdateExemplars(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zaptest.NewLogger(t))
	traces := buildSampleTrace()
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	traceID := span.TraceID()
	spanID := span.SpanID()
	key := metricKey("metricKey")
	value := float64(42)

	conn.updateHistogram(key, value, traceID, spanID)

	require.NotEmpty(t, conn.histograms[key].exemplarsData)
	assert.Equal(t, exemplarData{traceID: traceID, spanID: spanID, value: value}, conn.histograms[key].exemplarsData[0])

	conn.resetExemplarData()
	assert.Empty(t, conn.histograms[key].exemplarsData)
}

func TestBuildKeyWithDimensionsOverflow(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.MaxServicesToTrack = 2
	cfg.MaxOperationsToTrackPerService = 2

	observedCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedCore)

	conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), logger)
	resAttr := pcommon.NewMap()

	for i := 0; i <= conn.maxNumberOfServicesToTrack; i++ {
		span0 := ptrace.NewSpan()
		span0.SetName("span")
		buf := &bytes.Buffer{}
		serviceName := fmt.Sprintf("service-%d", i)
		conn.buildKey(buf, serviceName, span0, []dimension{}, resAttr)
	}

	span0 := ptrace.NewSpan()
	span0.SetName("span-overflow")
	buf := &bytes.Buffer{}
	serviceName := fmt.Sprintf("service-%d", conn.maxNumberOfServicesToTrack)
	conn.buildKey(buf, serviceName, span0, []dimension{}, resAttr)
	assert.Contains(t, buf.String(), overflowServiceName)

	foundServiceOverflowLog := false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Too many services to track") {
			foundServiceOverflowLog = true
			break
		}
	}
	assert.True(t, foundServiceOverflowLog)

	conn.serviceToOperations = make(map[string]map[string]struct{})
	conn.buildKey(buf, "simple_service", span0, []dimension{}, resAttr)

	for i := 0; i <= conn.maxNumberOfOperationsToTrackPerService; i++ {
		spanOp := ptrace.NewSpan()
		spanOp.SetName(fmt.Sprintf("operation-%d", i))
		buf := &bytes.Buffer{}
		conn.buildKey(buf, "simple_service", spanOp, []dimension{}, resAttr)
	}

	spanOverflow := ptrace.NewSpan()
	spanOverflow.SetName(fmt.Sprintf("operation-%d", conn.maxNumberOfOperationsToTrackPerService))
	buf = &bytes.Buffer{}
	conn.buildKey(buf, "simple_service", spanOverflow, []dimension{}, resAttr)
	assert.Contains(t, buf.String(), overflowOperation)

	foundOperationOverflowLog := false
	for _, log := range observedLogs.All() {
		if strings.Contains(log.Message, "Too many operations to track") {
			foundOperationOverflowLog = true
			break
		}
	}
	assert.True(t, foundOperationOverflowLog)
}

func TestBuildMetricKeyConditionalTimeBucketing(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 12, 30, 15, 0, time.UTC)
	timeBucketInterval := time.Minute
	resourceAttr := pcommon.NewMap()
	serviceName := "test-service"

	span := ptrace.NewSpan()
	span.SetName("test-operation")
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(100 * time.Millisecond)))

	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.AggregationTemporality = delta
	cfg.TimeBucketInterval = timeBucketInterval

	deltaConn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
	deltaKey := deltaConn.buildMetricKey(serviceName, span, nil, resourceAttr)
	expectedBucket := startTime.Truncate(timeBucketInterval).Unix()
	assert.True(t, strings.HasPrefix(string(deltaKey), strconv.FormatInt(expectedBucket, 10)))

	cfg.AggregationTemporality = cumulative
	cumulativeConn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
	cumulativeKey := cumulativeConn.buildMetricKey(serviceName, span, nil, resourceAttr)
	assert.False(t, strings.HasPrefix(string(cumulativeKey), strconv.FormatInt(expectedBucket, 10)))
	assert.True(t, strings.HasPrefix(string(cumulativeKey), serviceName))
}

func TestBuildCustomMetricKeyConditionalTimeBucketing(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 12, 30, 15, 0, time.UTC)
	timeBucketInterval := time.Minute
	resourceAttr := pcommon.NewMap()
	serviceName := "test-service"
	extraVals := []string{"example.com:8080"}

	span := ptrace.NewSpan()
	span.SetName("test-operation")
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(100 * time.Millisecond)))

	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.AggregationTemporality = delta
	cfg.TimeBucketInterval = timeBucketInterval

	deltaConn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
	deltaKey := deltaConn.buildCustomMetricKey(serviceName, span, nil, resourceAttr, extraVals)
	expectedBucket := startTime.Truncate(timeBucketInterval).Unix()
	assert.True(t, strings.HasPrefix(string(deltaKey), strconv.FormatInt(expectedBucket, 10)))

	cfg.AggregationTemporality = cumulative
	cumulativeConn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
	cumulativeKey := cumulativeConn.buildCustomMetricKey(serviceName, span, nil, resourceAttr, extraVals)
	assert.False(t, strings.HasPrefix(string(cumulativeKey), strconv.FormatInt(expectedBucket, 10)))
	assert.True(t, strings.HasPrefix(string(cumulativeKey), serviceName))
}

func TestBuildMetricsTimestampAccuracy(t *testing.T) {
	bucketStart := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)

	t.Run("Delta_Uses_Bucket_Timestamps", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		configureTestDimensions(cfg)
		cfg.AggregationTemporality = delta
		cfg.TimeBucketInterval = time.Minute

		conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
		conn.config.SkipSpansOlderThan = 100 * 365 * 24 * time.Hour

		resourceAttr := pcommon.NewMap()
		resourceAttr.PutStr(serviceNameKey, "test-service")

		span := ptrace.NewSpan()
		span.SetName("test-span")
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(bucketStart.Add(15 * time.Second)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(bucketStart.Add(45 * time.Second)))

		conn.aggregateMetricsForSpan("test-service", span, resourceAttr)

		metrics, err := conn.buildMetrics()
		require.NoError(t, err)
		require.Equal(t, 1, metrics.ResourceMetrics().Len())

		ilm := metrics.ResourceMetrics().At(0).ScopeMetrics()
		require.Equal(t, 1, ilm.Len())

		allMetrics := ilm.At(0).Metrics()
		require.Greater(t, allMetrics.Len(), 0)

		foundDeltaTimestamps := false
		for i := 0; i < allMetrics.Len(); i++ {
			metric := allMetrics.At(i)
			switch metric.Type() {
			case pmetric.MetricTypeHistogram:
				dps := metric.Histogram().DataPoints()
				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)
					if serviceAttr, ok := dp.Attributes().Get(serviceNameKey); ok && serviceAttr.Str() == "test-service" {
						start := dp.StartTimestamp().AsTime()
						end := dp.Timestamp().AsTime()
						assert.Equal(t, bucketStart, start)
						assert.Equal(t, bucketStart, end)
						foundDeltaTimestamps = true
					}
				}
			case pmetric.MetricTypeSum:
				dps := metric.Sum().DataPoints()
				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)
					if serviceAttr, ok := dp.Attributes().Get(serviceNameKey); ok && serviceAttr.Str() == "test-service" {
						start := dp.StartTimestamp().AsTime()
						end := dp.Timestamp().AsTime()
						assert.Equal(t, bucketStart, start)
						assert.Equal(t, bucketStart, end)
						foundDeltaTimestamps = true
					}
				}
			}
		}
		assert.True(t, foundDeltaTimestamps)
	})

	t.Run("Cumulative_Uses_Current_Time", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		configureTestDimensions(cfg)
		cfg.AggregationTemporality = cumulative
		cfg.TimeBucketInterval = time.Minute

		conn, _ := newTestConnectorInstance(t, cfg, clockwork.NewRealClock(), zap.NewNop())
		conn.config.SkipSpansOlderThan = 100 * 365 * 24 * time.Hour

		resourceAttr := pcommon.NewMap()
		resourceAttr.PutStr(serviceNameKey, "test-service")

		span := ptrace.NewSpan()
		span.SetName("cumulative-span")
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(bucketStart.Add(15 * time.Second)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(bucketStart.Add(45 * time.Second)))

		conn.aggregateMetricsForSpan("test-service", span, resourceAttr)

		beforeBuild := time.Now()
		metrics, err := conn.buildMetrics()
		require.NoError(t, err)

		require.Equal(t, 1, metrics.ResourceMetrics().Len())
		ilm := metrics.ResourceMetrics().At(0).ScopeMetrics()
		require.Equal(t, 1, ilm.Len())

		allMetrics := ilm.At(0).Metrics()
		require.Greater(t, allMetrics.Len(), 0)

		foundCumulativeTimestamps := false
		for i := 0; i < allMetrics.Len(); i++ {
			metric := allMetrics.At(i)
			switch metric.Type() {
			case pmetric.MetricTypeHistogram:
				dps := metric.Histogram().DataPoints()
				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)
					if serviceAttr, ok := dp.Attributes().Get(serviceNameKey); ok && serviceAttr.Str() == "test-service" {
						start := dp.StartTimestamp().AsTime()
						end := dp.Timestamp().AsTime()
						assert.Equal(t, conn.startTimestamp.AsTime(), start)
						assert.True(t, end.After(beforeBuild.Add(-5*time.Second)) && end.Before(beforeBuild.Add(5*time.Second)))
						foundCumulativeTimestamps = true
					}
				}
			case pmetric.MetricTypeSum:
				dps := metric.Sum().DataPoints()
				for j := 0; j < dps.Len(); j++ {
					dp := dps.At(j)
					if serviceAttr, ok := dp.Attributes().Get(serviceNameKey); ok && serviceAttr.Str() == "test-service" {
						start := dp.StartTimestamp().AsTime()
						end := dp.Timestamp().AsTime()
						assert.Equal(t, conn.startTimestamp.AsTime(), start)
						assert.True(t, end.After(beforeBuild.Add(-5*time.Second)) && end.Before(beforeBuild.Add(5*time.Second)))
						foundCumulativeTimestamps = true
					}
				}
			}
		}
		assert.True(t, foundCumulativeTimestamps)
	})
}

func TestSkipSpansOlderThan(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureTestDimensions(cfg)
	cfg.TimeBucketInterval = time.Minute

	resourceAttr := pcommon.NewMap()
	resourceAttr.PutStr(serviceNameKey, "svc")
	serviceName := "svc"
	window := 24 * time.Hour
	now := time.Now()

	tests := []struct {
		name           string
		temporality    string
		spanStart      time.Time
		expectAccepted bool
	}{
		{
			name:           "Delta_Skips_Stale",
			temporality:    delta,
			spanStart:      now.Add(-window - time.Hour),
			expectAccepted: false,
		},
		{
			name:           "Delta_Accepts_Recent",
			temporality:    delta,
			spanStart:      now.Add(-window).Add(time.Millisecond),
			expectAccepted: true,
		},
		{
			name:           "Cumulative_Skips_Stale",
			temporality:    cumulative,
			spanStart:      now.Add(-window - 2*time.Hour),
			expectAccepted: false,
		},
		{
			name:           "Cumulative_Accepts_Recent",
			temporality:    cumulative,
			spanStart:      now.Add(-window).Add(time.Millisecond),
			expectAccepted: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfgCopy := *cfg
			cfgCopy.AggregationTemporality = tc.temporality
			conn, _ := newTestConnectorInstance(t, &cfgCopy, clockwork.NewRealClock(), zap.NewNop())
			conn.config.SkipSpansOlderThan = window

			span := ptrace.NewSpan()
			span.SetName("span")
			span.SetKind(ptrace.SpanKindServer)
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(tc.spanStart))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(tc.spanStart.Add(10 * time.Millisecond)))

			conn.aggregateMetricsForSpan(serviceName, span, resourceAttr)

			if tc.expectAccepted {
				assert.NotEmpty(t, conn.histograms)
				assert.NotEmpty(t, conn.callHistograms)
			} else {
				assert.Empty(t, conn.histograms)
				assert.Empty(t, conn.callHistograms)
			}
		})
	}
}

func configureTestDimensions(cfg *Config) {
	defaultNull := defaultNullValue
	defaultNotInSpan := defaultNotInSpanAttrVal
	cfg.Dimensions = []Dimension{
		{Name: stringAttrName},
		{Name: intAttrName},
		{Name: doubleAttrName},
		{Name: boolAttrName},
		{Name: mapAttrName},
		{Name: arrayAttrName},
		{Name: nullAttrName, Default: &defaultNull},
		{Name: notInSpanAttrName0, Default: &defaultNotInSpan},
		{Name: notInSpanAttrName1},
		{Name: conflictResourceAttr},
		{Name: regionResourceAttrName},
	}
	cfg.DimensionsCacheSize = DimensionsCacheSize
}

func newTestConnectorInstance(t *testing.T, cfg *Config, clk clockwork.Clock, logger *zap.Logger) (*connectorImp, *consumertest.MetricsSink) {
	t.Helper()
	conn, err := newConnector(logger, cfg, clk, testID)
	require.NoError(t, err)
	sink := &consumertest.MetricsSink{}
	conn.metricsConsumer = sink
	return conn, sink
}
