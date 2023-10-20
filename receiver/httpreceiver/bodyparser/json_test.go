package bodyparser

import (
	"encoding/hex"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestJSONLogParser(t *testing.T) {
	t.Parallel()
	d := NewJSON()
	tests := []struct {
		name    string
		PayLoad string
		Logs    func() plog.Logs
		count   int
		isError bool
	}{
		{
			name:    "Test 1",
			PayLoad: `[{"body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				return ld
			},
		},
		{
			name:    "Test 2  - wrong structure",
			PayLoad: `{"body":"hello world"}`,
			isError: true,
		},
		{
			name:    "Test 3 - proper trace_id and span_id",
			PayLoad: `[{"trace_id": "000000000000000045f6f14f5b4cc85a", "span_id": "1010f0feffbfeb95", "body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				//trace
				traceIdByte, _ := hex.DecodeString("000000000000000045f6f14f5b4cc85a")
				var traceID [16]byte
				copy(traceID[:], traceIdByte)
				log.SetTraceID(pcommon.TraceID(traceID))
				// span
				spanIdByte, _ := hex.DecodeString("1010f0feffbfeb95")
				var spanID [8]byte
				copy(spanID[:], spanIdByte)
				log.SetSpanID(pcommon.SpanID(spanID))
				return ld
			},
		},
		{
			name:    "Test 3 - incorrect trace_id",
			PayLoad: `[{"trace_id": "0000000000045f6f14f5b4cc85a", "body":"hello world"}]`,
			isError: true,
		},
		{
			name:    "Test 3 - incorrect span",
			PayLoad: `[{"span_id": "1010f0feffsbfeb95", "body":"hello world"}]`,
			isError: true,
		},
		{
			name:    "Test 4 - attributes",
			PayLoad: `[{"attributes": {"str": "hello", "int": 10, "float": 10.0, "boolean": true, "map": {"ab": "cd"}, "slice": ["x1", "x2"]}, "body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				log.Attributes().EnsureCapacity(6)
				log.Attributes().PutStr("str", "hello")
				log.Attributes().PutDouble("int", 10)
				log.Attributes().PutDouble("float", 10.0)
				log.Attributes().PutBool("boolean", true)
				log.Attributes().PutStr("map", `{"ab":"cd"}`)
				log.Attributes().PutStr("slice", `["x1","x2"]`)
				return ld
			},
		},
		{
			name:    "Test 5 - resources",
			PayLoad: `[{"resources": {"str": "hello", "int": 10, "float": 10.0, "boolean": true, "map": {"ab": "cd"}, "slice": ["x1", "x2"]}, "body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().EnsureCapacity(6)
				rl.Resource().Attributes().PutStr("str", "hello")
				rl.Resource().Attributes().PutDouble("int", 10)
				rl.Resource().Attributes().PutDouble("float", 10.0)
				rl.Resource().Attributes().PutBool("boolean", true)
				rl.Resource().Attributes().PutStr("map", `{"ab":"cd"}`)
				rl.Resource().Attributes().PutStr("slice", `["x1","x2"]`)
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				return ld
			},
		},
		{
			name:    "Test 6 - severity",
			PayLoad: `[{"severity_text": "info", "severity_number": 9, "body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				log.Attributes().EnsureCapacity(2)
				log.SetSeverityText("info")
				log.SetSeverityNumber(9)
				return ld
			},
		},
		{
			name:    "Test 7 - flags",
			PayLoad: `[{"trace_flags": 1, "body":"hello world"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				log.Attributes().EnsureCapacity(2)
				log.SetFlags(plog.LogRecordFlags(1))
				return ld
			},
		},
		{
			name:    "Test 8 - multiple logs",
			PayLoad: `[{"body":"hello world"}, {"body":"hello world 2"}]`,
			Logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				sl := rl.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				log.Body().SetStr("hello world")
				rl1 := ld.ResourceLogs().AppendEmpty()
				sl1 := rl1.ScopeLogs().AppendEmpty()
				log1 := sl1.LogRecords().AppendEmpty()
				log1.Body().SetStr("hello world 2")
				return ld
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _, err := d.Parse([]byte(tt.PayLoad))
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, plogtest.CompareLogs(tt.Logs(), res, plogtest.IgnoreObservedTimestamp()))
			}
		})
	}
}
