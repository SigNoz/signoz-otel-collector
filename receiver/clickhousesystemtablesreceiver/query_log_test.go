package clickhousesystemtablesreceiver

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestQueryLogToLogRecord(t *testing.T) {
	testCases := []struct {
		name                   string
		ql                     QueryLog
		expectedSeverityNumber plog.SeverityNumber
		expectedSeverityText   string
		expectedTimestamp      pcommon.Timestamp
		expectedBody           string
		expectedAttributes     map[string]interface{}
	}{
		{
			name: "Normal Query",
			ql: QueryLog{
				EventType:             "QueryFinish",
				EventTimeMicroseconds: time.Date(2023, 10, 31, 12, 0, 0, 0, time.UTC),
				Query:                 "SELECT * FROM table",
			},
			expectedSeverityNumber: plog.SeverityNumberInfo,
			expectedSeverityText:   "INFO",
			expectedTimestamp:      pcommon.NewTimestampFromTime(time.Date(2023, 10, 31, 12, 0, 0, 0, time.UTC)),
			expectedBody:           "SELECT * FROM table",
			expectedAttributes: map[string]interface{}{
				"source":                     "clickhouse",
				"clickhouse.query_log.type":  "QueryFinish",
				"clickhouse.query_log.query": "SELECT * FROM table",
				"clickhouse.query_log.event_time_microseconds": time.Date(2023, 10, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		},
		{
			name: "Exception Query",
			ql: QueryLog{
				EventType:             "ExceptionWhileProcessing",
				EventTimeMicroseconds: time.Date(2023, 10, 31, 13, 0, 0, 0, time.UTC),
				Query:                 "SELECT * FROM invalid_table",
			},
			expectedSeverityNumber: plog.SeverityNumberError,
			expectedSeverityText:   "ERROR",
			expectedTimestamp:      pcommon.NewTimestampFromTime(time.Date(2023, 10, 31, 13, 0, 0, 0, time.UTC)),
			expectedBody:           "SELECT * FROM invalid_table",
			expectedAttributes: map[string]interface{}{
				"source":                     "clickhouse",
				"clickhouse.query_log.type":  "ExceptionWhileProcessing",
				"clickhouse.query_log.query": "SELECT * FROM invalid_table",
				"clickhouse.query_log.event_time_microseconds": time.Date(2023, 10, 31, 13, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		},
		{
			name: "Query with Slices and Maps",
			ql: QueryLog{
				EventType:             "QueryFinish",
				EventTimeMicroseconds: time.Date(2023, 10, 31, 14, 0, 0, 0, time.UTC),
				Query:                 "SELECT * FROM table",
				Databases:             []string{"db1", "db2"},
				Tables:                []string{"table1", "table2"},
				ProfileEvents:         map[string]uint64{"event1": 10, "event2": 20},
			},
			expectedSeverityNumber: plog.SeverityNumberInfo,
			expectedSeverityText:   "INFO",
			expectedTimestamp:      pcommon.NewTimestampFromTime(time.Date(2023, 10, 31, 14, 0, 0, 0, time.UTC)),
			expectedBody:           "SELECT * FROM table",
			expectedAttributes: map[string]interface{}{
				"source":                                       "clickhouse",
				"clickhouse.query_log.type":                    "QueryFinish",
				"clickhouse.query_log.query":                   "SELECT * FROM table",
				"clickhouse.query_log.databases":               "db1,db2",
				"clickhouse.query_log.tables":                  "table1,table2",
				"clickhouse.query_log.event_time_microseconds": time.Date(2023, 10, 31, 14, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"clickhouse.query_log.ProfileEvents.event1":    uint64(10),
				"clickhouse.query_log.ProfileEvents.event2":    uint64(20),
			},
		},
		{
			name: "Log Comment",
			ql: QueryLog{
				EventType:             "QueryFinish",
				EventTimeMicroseconds: time.Date(2023, 10, 31, 15, 0, 0, 0, time.UTC),
				Query:                 "SELECT * FROM table",
				LogComment:            `{"key1":"value1","key2":"value2"}`,
			},
			expectedSeverityNumber: plog.SeverityNumberInfo,
			expectedSeverityText:   "INFO",
			expectedTimestamp:      pcommon.NewTimestampFromTime(time.Date(2023, 10, 31, 15, 0, 0, 0, time.UTC)),
			expectedBody:           "SELECT * FROM table",
			expectedAttributes: map[string]interface{}{
				"clickhouse.query_log.log_comment.key1": "value1",
				"clickhouse.query_log.log_comment.key2": "value2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lr, err := tc.ql.toLogRecord()
			if err != nil {
				t.Fatalf("toLogRecord returned error: %v", err)
			}

			if lr.SeverityNumber() != tc.expectedSeverityNumber {
				t.Errorf("SeverityNumber mismatch. Got %v, expected %v", lr.SeverityNumber(), tc.expectedSeverityNumber)
			}

			if lr.SeverityText() != tc.expectedSeverityText {
				t.Errorf("SeverityText mismatch. Got %v, expected %v", lr.SeverityText(), tc.expectedSeverityText)
			}

			if lr.Timestamp() != tc.expectedTimestamp {
				t.Errorf("Timestamp mismatch. Got %v, expected %v", lr.Timestamp().AsTime(), tc.expectedTimestamp.AsTime())
			}

			if lr.Body().Type() != pcommon.ValueTypeStr {
				t.Errorf("Body type mismatch. Expected string, got %v", lr.Body().Type())
			} else if lr.Body().Str() != tc.expectedBody {
				t.Errorf("Body mismatch. Got %v, expected %v", lr.Body().Str(), tc.expectedBody)
			}

			// Check attributes
			attrs := lr.Attributes()

			for key := range tc.expectedAttributes {
				expectedValue := tc.expectedAttributes[key]
				attrVal, ok := attrs.Get(key)
				if !ok {
					t.Errorf("Expected attribute key not found: %s", key)
				}
				if !compareAttributeValue(attrVal, expectedValue) {
					t.Errorf("Attribute %s mismatch. Got %v, expected %v", key, attrVal.AsRaw(), expectedValue)
				}
			}
		})
	}
}

func compareAttributeValue(attrVal pcommon.Value, expectedValue interface{}) bool {
	switch attrVal.Type() {
	case pcommon.ValueTypeStr:
		expectedStr, ok := expectedValue.(string)
		if !ok {
			return false
		}
		return attrVal.Str() == expectedStr
	case pcommon.ValueTypeInt:
		switch v := expectedValue.(type) {
		case int64:
			return attrVal.Int() == v
		case uint64:
			return uint64(attrVal.Int()) == v
		case int:
			return attrVal.Int() == int64(v)
		default:
			return false
		}
	case pcommon.ValueTypeDouble:
		expectedFloat, ok := expectedValue.(float64)
		if !ok {
			return false
		}
		return attrVal.Double() == expectedFloat
	default:
		// For simplicity, only handle string, int, double types
		return false
	}
}
