// Package filterrule holds the rules for telemetry the ClickHouse exporters never write.
package filterrule

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func OldestAllowed(now time.Time, maxAge time.Duration) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(now.Add(-maxAge))
}

// LogRecordTooOld uses Timestamp, falling back to ObservedTimestamp. A record with neither is never too old.
func LogRecordTooOld(record plog.LogRecord, oldestAllowed pcommon.Timestamp) bool {
	ts := record.Timestamp()
	if ts == 0 {
		ts = record.ObservedTimestamp()
	}
	return ts != 0 && ts < oldestAllowed
}

func SpanTooOld(span ptrace.Span, oldestAllowed pcommon.Timestamp) bool {
	return span.StartTimestamp() < oldestAllowed
}
