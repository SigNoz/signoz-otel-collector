package filterrule

import (
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/ptracesgen"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestOldestAllowed(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)

	assert.Equal(t, pcommon.NewTimestampFromTime(time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC)), OldestAllowed(now, 15*24*time.Hour))
}

func TestLogRecordTooOld(t *testing.T) {
	oldestAllowed := pcommon.Timestamp(1000)

	testCases := []struct {
		name              string
		timestamp         pcommon.Timestamp
		observedTimestamp pcommon.Timestamp
		expected          bool
	}{
		{
			name:              "OldTimestamp_RecentObserved_Dropped",
			timestamp:         999, // Timestamp wins over ObservedTimestamp: 999 < 1000, so dropped
			observedTimestamp: 2000,
			expected:          true,
		},
		{
			name:      "TimestampAtOldestAllowed_Kept",
			timestamp: 1000, // oldestAllowed itself is allowed: 1000 is not < 1000, so kept
			expected:  false,
		},
		{
			name:              "RecentTimestamp_OldObserved_Kept",
			timestamp:         2000, // Timestamp wins over ObservedTimestamp: 2000 >= 1000, so kept
			observedTimestamp: 999,
			expected:          false,
		},
		{
			name:              "MissingTimestamp_OldObserved_Dropped",
			observedTimestamp: 999, // no Timestamp, so ObservedTimestamp is used: 999 < 1000, so dropped
			expected:          true,
		},
		{
			name:              "MissingTimestamp_RecentObserved_Kept",
			observedTimestamp: 2000, // no Timestamp, so ObservedTimestamp is used: 2000 >= 1000, so kept
			expected:          false,
		},
		{
			name:     "MissingBothTimestamps_Kept",
			expected: false, // no timestamp to compare, so kept
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ld := plogsgen.Generate(plogsgen.WithTimestamp(testCase.timestamp), plogsgen.WithObservedTimestamp(testCase.observedTimestamp))
			record := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

			assert.Equal(t, testCase.expected, LogRecordTooOld(record, oldestAllowed))
		})
	}
}

func TestSpanTooOld(t *testing.T) {
	oldestAllowed := pcommon.Timestamp(1000)

	testCases := []struct {
		name           string
		startTimestamp pcommon.Timestamp
		expected       bool
	}{
		{
			name:           "OldStart_Dropped",
			startTimestamp: 999, // 999 < 1000, so dropped
			expected:       true,
		},
		{
			name:           "StartAtOldestAllowed_Kept",
			startTimestamp: 1000, // oldestAllowed itself is allowed: 1000 is not < 1000, so kept
			expected:       false,
		},
		{
			name:           "RecentStart_Kept",
			startTimestamp: 2000, // 2000 >= 1000, so kept
			expected:       false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			td := ptracesgen.Generate(ptracesgen.WithStartTimestamp(testCase.startTimestamp))
			span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

			assert.Equal(t, testCase.expected, SpanTooOld(span, oldestAllowed))
		})
	}
}
