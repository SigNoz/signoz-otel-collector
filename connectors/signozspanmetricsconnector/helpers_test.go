package signozspanmetricsconnector

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

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
