package signozfilterprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/ptracesgen"
)

func newTestProcessor() *filterProcessor {
	return newProcessor(createDefaultConfig().(*Config))
}

func daysAgo(days int) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.Now().Add(-time.Duration(days) * 24 * time.Hour))
}

func TestProcessLogs(t *testing.T) {
	type record struct {
		timestamp         pcommon.Timestamp
		observedTimestamp pcommon.Timestamp
	}

	testCases := []struct {
		name        string
		records     []record
		expected    int
		expectedErr error
	}{
		{
			name: "OldAndRecent_OldDropped",
			records: []record{
				{timestamp: daysAgo(30)}, // dropped
				{timestamp: daysAgo(1)},  // kept
				{timestamp: daysAgo(16)}, // dropped
			},
			expected: 1,
		},
		{
			name: "MissingTimestamp_UsesObserved_OldDropped",
			records: []record{
				{observedTimestamp: daysAgo(30)}, // dropped
				{observedTimestamp: daysAgo(1)},  // kept
			},
			expected: 1,
		},
		{
			name: "MissingBothTimestamps_Kept",
			records: []record{
				{}, // kept
			},
			expected: 1,
		},
		{
			name: "AllOld_AllDropped",
			records: []record{
				{timestamp: daysAgo(30)}, // dropped
			},
			expected:    0,
			expectedErr: processorhelper.ErrSkipProcessingData, // nothing left, so nothing is passed on
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ld := plog.NewLogs()
			for _, r := range testCase.records {
				plogsgen.Generate(plogsgen.WithTimestamp(r.timestamp), plogsgen.WithObservedTimestamp(r.observedTimestamp)).
					ResourceLogs().MoveAndAppendTo(ld.ResourceLogs())
			}

			got, err := newTestProcessor().ProcessLogs(context.Background(), ld)
			assert.ErrorIs(t, err, testCase.expectedErr)
			assert.Equal(t, testCase.expected, got.LogRecordCount())
		})
	}
}

func TestProcessTraces(t *testing.T) {
	testCases := []struct {
		name            string
		startTimestamps []pcommon.Timestamp
		expected        int
		expectedErr     error
	}{
		{
			name: "OldAndRecent_OldDropped",
			startTimestamps: []pcommon.Timestamp{
				daysAgo(30), // dropped
				daysAgo(1),  // kept
				daysAgo(16), // dropped
			},
			expected: 1,
		},
		{
			name: "AllRecent_AllKept",
			startTimestamps: []pcommon.Timestamp{
				daysAgo(1),  // kept
				daysAgo(14), // kept
			},
			expected: 2,
		},
		{
			name: "AllOld_AllDropped",
			startTimestamps: []pcommon.Timestamp{
				daysAgo(30), // dropped
			},
			expected:    0,
			expectedErr: processorhelper.ErrSkipProcessingData, // nothing left, so nothing is passed on
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			td := ptrace.NewTraces()
			for _, ts := range testCase.startTimestamps {
				ptracesgen.Generate(ptracesgen.WithStartTimestamp(ts)).ResourceSpans().MoveAndAppendTo(td.ResourceSpans())
			}

			got, err := newTestProcessor().ProcessTraces(context.Background(), td)
			assert.ErrorIs(t, err, testCase.expectedErr)
			assert.Equal(t, testCase.expected, got.SpanCount())
		})
	}
}
