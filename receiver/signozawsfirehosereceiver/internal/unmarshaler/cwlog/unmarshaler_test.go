// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-json"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/cwlog/compression"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		filename          string
		wantResourceCount int
		wantLogCount      int
		wantErr           error
	}{
		"WithMultipleRecords": {
			filename:          "multiple_records",
			wantResourceCount: 1,
			wantLogCount:      2,
		},
		"WithSingleRecord": {
			filename:          "single_record",
			wantResourceCount: 1,
			wantLogCount:      1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithSomeInvalidRecords": {
			filename:          "some_invalid_records",
			wantResourceCount: 1,
			wantLogCount:      2,
		},
		"WithMultipleResources": {
			filename:          "multiple_resources",
			wantResourceCount: 3,
			wantLogCount:      6,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := compression.Zip(record)
			require.NoError(t, err)
			records := [][]byte{compressedRecord}

			got, err := unmarshaler.Unmarshal(records)
			if testCase.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, testCase.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceLogs().Len())
				gotLogCount := 0
				for i := 0; i < got.ResourceLogs().Len(); i++ {
					rm := got.ResourceLogs().At(i)
					require.Equal(t, 1, rm.ScopeLogs().Len())
					ilm := rm.ScopeLogs().At(0)
					gotLogCount += ilm.LogRecords().Len()
				}
				require.Equal(t, testCase.wantLogCount, gotLogCount)
			}
		})
	}
}

func TestCWLogTimestampUnmarshaling(t *testing.T) {
	// CW log records have timestamp in milliseconds
	// while the otlp log model expects timestamps in nanos.
	// ensure timestamps are unmarshaled correctly
	require := require.New(t)

	testRecordBytes, err := os.ReadFile(filepath.Join(".", "testdata", "single_record"))
	require.NoError(err)

	compressedTestRecord, err := compression.Zip(testRecordBytes)
	require.NoError(err)

	unmarshaler := NewUnmarshaler(zap.NewNop())
	unmarshaledPlogs, err := unmarshaler.Unmarshal([][]byte{compressedTestRecord})
	require.NoError(err)

	require.Equal(1, unmarshaledPlogs.ResourceLogs().Len())
	rLogs := unmarshaledPlogs.ResourceLogs().At(0)
	require.Equal(1, rLogs.ScopeLogs().Len())
	sLogs := rLogs.ScopeLogs().At(0)
	require.Equal(1, sLogs.LogRecords().Len())
	unmarshaledLogRecord := sLogs.LogRecords().At(0)

	// extract timestamp present in test data
	// and validate it matches the unmarshaled log record.
	testCWLogsRecord := cWLog{}
	err = json.Unmarshal(testRecordBytes, &testCWLogsRecord)
	require.NoError(err)

	require.Equal(1, len(testCWLogsRecord.LogEvents))
	testCwLogEvent := testCWLogsRecord.LogEvents[0]

	require.Equal(
		unmarshaledLogRecord.Timestamp().AsTime().UnixMilli(),
		testCwLogEvent.Timestamp, // CW Log events have timestamp in milliseconds
	)
}
