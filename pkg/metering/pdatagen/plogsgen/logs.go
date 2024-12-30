package plogsgen

import (
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Generate(opts ...GenerationOption) plog.Logs {
	generationOpts := generationOptions{
		logRecordCount:               1,
		resourceAttributeCount:       1,
		body:                         "This is a test log record",
		resourceAttributeStringValue: "resource",
	}

	for _, opt := range opts {
		opt(&generationOpts)
	}

	endTime := pcommon.NewTimestampFromTime(time.Now())
	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()
	for i := 0; i < generationOpts.resourceAttributeCount; i++ {
		suffix := strconv.Itoa(i)
		// Do not change the key name format in resource attributes below.
		resourceLog.Resource().Attributes().PutStr("resource."+suffix, generationOpts.resourceAttributeStringValue)
	}

	scopeLogs := resourceLog.ScopeLogs().AppendEmpty()
	scopeLogs.LogRecords().EnsureCapacity(generationOpts.logRecordCount)
	for i := 0; i < generationOpts.logRecordCount; i++ {
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(endTime)
		logRecord.SetObservedTimestamp(endTime)
		logRecord.Body().SetStr(generationOpts.body)
	}

	return logs
}
