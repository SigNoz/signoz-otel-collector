// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/cwlog"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

// resourceAttributes are the CloudWatch log attributes that define a unique resource.
type resourceAttributes struct {
	owner, logGroup, logStream string
}

// resourceLogsBuilder provides convenient access to the a Resource's LogRecordSlice.
type resourceLogsBuilder struct {
	rls plog.LogRecordSlice
}

// setAttributes applies the resourceAttributes to the provided Resource.
func (ra *resourceAttributes) setAttributes(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudAccountID, ra.owner)
	attrs.PutStr(attributeAWSCloudWatchLogGroupName, ra.logGroup)
	attrs.PutStr(attributeAWSCloudWatchLogStreamName, ra.logStream)
}

// newResourceLogsBuilder to capture logs for the Resource defined by the provided attributes.
func newResourceLogsBuilder(logs plog.Logs, attrs resourceAttributes) *resourceLogsBuilder {
	rls := logs.ResourceLogs().AppendEmpty()
	attrs.setAttributes(rls.Resource())
	return &resourceLogsBuilder{rls.ScopeLogs().AppendEmpty().LogRecords()}
}

// AddLog events to the LogRecordSlice. Resource attributes are captured when creating
// the resourceLogsBuilder, so we only need to consider the LogEvents themselves.
func (rlb *resourceLogsBuilder) AddLog(log cWLog) {
	for _, event := range log.LogEvents {
		logLine := rlb.rls.AppendEmpty()

		logLine.SetTimestamp(pcommon.Timestamp(toEpochNano(event.Timestamp)))

		logLine.Body().SetStr(event.Message)
	}
}

// convert `epoch` of any precision to nanos
func toEpochNano(epoch int64) uint64 {
	epochCopy := epoch
	count := 0
	if epoch == 0 {
		count = 1
	} else {
		for epoch != 0 {
			epoch /= 10
			count++
		}
	}
	return uint64(epochCopy) * uint64(math.Pow(10, float64(19-count)))
}
