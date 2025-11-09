package signozspanmetricsconnector

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
			start, end := parseTimesFromKeyOrNow(tc.key, tc.now, tc.procStart)
			assert.Equal(t, pcommon.NewTimestampFromTime(tc.wantStart), start)
			assert.Equal(t, pcommon.NewTimestampFromTime(tc.wantEnd), end)
		})
	}
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
	type testcase struct {
		input             string
		skipSanitizeLabel bool
		expected          string
	}

	testCases := []testcase{
		// skipSanitizeLabel = false
		{"", false, ""},
		{"_test", false, "key_test"},
		{"__test", false, "key__test"},
		{"0test", false, "key_0test"},
		{"test", false, "test"},
		{"test_/", false, "test__"},
		// skipSanitizeLabel = true
		{"", true, ""},
		{"_test", true, "_test"},
		{"__test", true, "key__test"},
		{"0test", true, "key_0test"},
		{"test", true, "test"},
		{"test_/", true, "test__"},
	}

	for _, tc := range testCases {
		t.Run(
			tc.input+"_"+fmt.Sprint(tc.skipSanitizeLabel),
			func(t *testing.T) {
				got := sanitize(tc.input, tc.skipSanitizeLabel)
				require.Equal(t, tc.expected, got)
			},
		)
	}
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
