package clickhouse

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTTLParser_ParseTTLFromEngineFull(t *testing.T) {
	logger := zap.NewNop()
	parser := NewTTLParser(logger)

	tests := []struct {
		name       string
		engineFull string
		expected   uint64
	}{
		{
			name:       "toIntervalSecond - 30 days",
			engineFull: "ReplicatedReplacingMergeTree('/clickhouse/tables/a84a78ab-54b5-4b61-ab73-8b92b96d6871/{shard}', '{replica}') PARTITION BY toDate(unix_milli / 1000) ORDER BY (env, temporality, metric_name, fingerprint, unix_milli) TTL toDateTime(unix_milli / 1000) + toIntervalSecond(2592000) SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1",
			expected:   30,
		},
		{
			name:       "toIntervalDay - 7 days",
			engineFull: "MergeTree() PARTITION BY toDate(timestamp / 1000000000) ORDER BY (timestamp, id) TTL toDateTime(timestamp / 1000000000) + toIntervalDay(7) SETTINGS index_granularity = 8192",
			expected:   7,
		},
		{
			name:       "toIntervalHour - 48 hours = 2 days",
			engineFull: "MergeTree() TTL toDateTime(ts) + toIntervalHour(48) SETTINGS index_granularity = 8192",
			expected:   2,
		},
		{
			name:       "toIntervalWeek - 2 weeks = 14 days",
			engineFull: "MergeTree() TTL toDateTime(ts) + toIntervalWeek(2) SETTINGS index_granularity = 8192",
			expected:   14,
		},
		{
			name:       "toIntervalMonth - 3 months = 90 days",
			engineFull: "MergeTree() TTL toDateTime(ts) + toIntervalMonth(3) SETTINGS index_granularity = 8192",
			expected:   90,
		},
		{
			name:       "no TTL",
			engineFull: "MergeTree() PARTITION BY toDate(timestamp) ORDER BY (timestamp, id) SETTINGS index_granularity = 8192",
			expected:   0,
		},
		{
			name:       "empty string",
			engineFull: "",
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseTTLFromEngineFull(tt.engineFull)
			if result != tt.expected {
				t.Errorf("ParseTTLFromEngineFull() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTTLParser_GetTableTTLDays_WithMock(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)
	parser := NewTTLParser(observedLogger)

	tests := []struct {
		name                string
		database            string
		tableName           string
		setupMock           func(cmock.ClickConnMockCommon)
		expected            uint64
		expectedLogMessages []string
		expectedLogLevel    zapcore.Level
	}{
		{
			name:      "successful TTL query with 7 days",
			database:  "test_db",
			tableName: "test_table",
			setupMock: func(mock cmock.ClickConnMockCommon) {
				columns := []cmock.ColumnType{
					{Name: "engine_full", Type: column.Type("String")},
				}
				values := [][]any{
					{"MergeTree() TTL toDateTime(ts) + toIntervalDay(7) SETTINGS index_granularity = 8192"},
				}
				rows := cmock.NewRows(columns, values)
				mock.ExpectSelect("SELECT engine_full FROM system.tables WHERE database = 'test_db' AND name = 'test_table'").
					WillReturnRows(rows)
			},
			expected:            7,
			expectedLogMessages: []string{"TTL configured for table"},
			expectedLogLevel:    zapcore.InfoLevel,
		},
		{
			name:      "successful TTL query with 30 days from seconds",
			database:  "metrics_db",
			tableName: "samples",
			setupMock: func(mock cmock.ClickConnMockCommon) {
				columns := []cmock.ColumnType{
					{Name: "engine_full", Type: column.Type("String")},
				}
				values := [][]any{
					{"ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}', '{replica}') PARTITION BY toDate(unix_milli / 1000) ORDER BY (env, temporality, metric_name, fingerprint, unix_milli) TTL toDateTime(unix_milli / 1000) + toIntervalSecond(2592000) SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1"},
				}
				rows := cmock.NewRows(columns, values)
				mock.ExpectSelect("SELECT engine_full FROM system.tables WHERE database = 'metrics_db' AND name = 'samples'").
					WillReturnRows(rows)
			},
			expected:            30, // 2592000s = 30d
			expectedLogMessages: []string{"TTL configured for table"},
			expectedLogLevel:    zapcore.InfoLevel,
		},
		{
			name:      "table not found",
			database:  "nonexistent_db",
			tableName: "nonexistent_table",
			setupMock: func(mock cmock.ClickConnMockCommon) {
				columns := []cmock.ColumnType{
					{Name: "engine_full", Type: column.Type("String")},
				}
				values := [][]any{}
				rows := cmock.NewRows(columns, values)
				mock.ExpectSelect("SELECT engine_full FROM system.tables WHERE database = 'nonexistent_db' AND name = 'nonexistent_table'").
					WillReturnRows(rows)
			},
			expected:            0,
			expectedLogMessages: []string{"table not found in system.tables"},
			expectedLogLevel:    zapcore.WarnLevel,
		},
		{
			name:      "no TTL configured",
			database:  "test_db",
			tableName: "no_ttl_table",
			setupMock: func(mock cmock.ClickConnMockCommon) {
				columns := []cmock.ColumnType{
					{Name: "engine_full", Type: column.Type("String")},
				}
				values := [][]any{
					{"MergeTree() PARTITION BY toDate(timestamp) ORDER BY (timestamp, id) SETTINGS index_granularity = 8192"},
				}
				rows := cmock.NewRows(columns, values)
				mock.ExpectSelect("SELECT engine_full FROM system.tables WHERE database = 'test_db' AND name = 'no_ttl_table'").
					WillReturnRows(rows)
			},
			expected:            0,
			expectedLogMessages: []string{"TTL not configured for table"},
			expectedLogLevel:    zapcore.InfoLevel,
		},
		{
			name:      "database error",
			database:  "error_db",
			tableName: "error_table",
			setupMock: func(mock cmock.ClickConnMockCommon) {
				mock.ExpectSelect("SELECT engine_full FROM system.tables WHERE database = 'error_db' AND name = 'error_table'").
					WillReturnError(errors.New("database connection failed"))
			},
			expected:            0,
			expectedLogMessages: []string{"error while fetching ttl from database"},
			expectedLogLevel:    zapcore.ErrorLevel,
		},
		{
			name:                "nil connection",
			database:            "test_db",
			tableName:           "test_table",
			setupMock:           func(mock cmock.ClickConnMockCommon) {},
			expected:            0,
			expectedLogMessages: []string{"no database connection available, cannot determine TTL"},
			expectedLogLevel:    zapcore.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogs.TakeAll()

			var conn clickhouse.Conn

			if tt.name != "nil connection" {
				mockConn, err := cmock.NewClickHouseNative(nil)
				require.NoError(t, err)
				tt.setupMock(mockConn)
				conn = mockConn
				defer func() {
					err := mockConn.ExpectationsWereMet()
					require.NoError(t, err)
				}()
			}

			result := parser.GetTableTTLDays(context.Background(), conn, tt.database, tt.tableName)
			require.Equal(t, tt.expected, result)

			logs := observedLogs.All()
			require.NotEmpty(t, logs, "Expected logs but got none")

			foundExpectedLog := false
			for _, log := range logs {
				for _, expectedMsg := range tt.expectedLogMessages {
					if log.Level == tt.expectedLogLevel &&
						len(log.Message) > 0 &&
						(log.Message == expectedMsg || (len(expectedMsg) > 0 && log.Message[:len(expectedMsg)] == expectedMsg)) {
						foundExpectedLog = true
						break
					}
				}
				if foundExpectedLog {
					break
				}
			}
			require.True(t, foundExpectedLog, "Expected log message '%v' with level %v not found in logs: %v",
				tt.expectedLogMessages, tt.expectedLogLevel, logs)
		})
	}
}
