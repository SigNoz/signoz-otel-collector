package signozdeduplicationidprocessor

import (
	"context"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/internal/coreinternal/testdata"
	"github.com/SigNoz/signoz-otel-collector/processor/signozdeduplicationidprocessor/internal/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestConsumeTraces(t *testing.T) {
	processor := &deduplicationIdProcessor{
		IdGenerator: id.NewConstantGenerator("test"),
	}

	tests := []struct {
		name      string
		id        string
		generator func() ptrace.Traces
		err       error
	}{
		{
			name:      "TracesOneEmptyResourceSpans",
			id:        "test",
			generator: func() ptrace.Traces { return testdata.GenerateTracesOneEmptyResourceSpans() },
			err:       nil,
		},
		{
			name:      "TracesOneEmptyResourceSpans",
			id:        "test",
			generator: func() ptrace.Traces { return testdata.GenerateTracesOneSpan() },
			err:       nil,
		},
		{
			name:      "TracesTwoSpansSameResourceOneDifferent",
			id:        "test",
			generator: func() ptrace.Traces { return testdata.GenerateTracesTwoSpansSameResourceOneDifferent() },
			err:       nil,
		},
		{
			name: "TracesOneSpanWithDeduplicationId",
			id:   "my-custom-id",
			generator: func() ptrace.Traces {
				td := testdata.GenerateTracesOneSpan()
				td.ResourceSpans().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: nil,
		},
		{
			name: "GenerateTracesTwoSpansSameResourceOneDifferentWithDeduplicationId",
			id:   "my-custom-id",
			generator: func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				td.ResourceSpans().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceSpans().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: nil,
		},
		{
			name: "GenerateTracesTwoSpansSameResourceOneDifferentWithDifferentDeduplicationId",
			id:   "",
			generator: func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				td.ResourceSpans().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceSpans().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id-2")
				return td
			},
			err: errTooManyDeduplicationIds,
		},
		{
			name: "GenerateTracesTwoSpansSameResourceOneDifferentWithMissingDeduplicationId",
			id:   "",
			generator: func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				td.ResourceSpans().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: errMissingDepuplicationId,
		},
		{
			name: "GenerateTracesThreeResourceWithDifferentAndMissingDeduplicationId",
			id:   "",
			generator: func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				td.ResourceSpans().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceSpans().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id-2")
				td.ResourceSpans().AppendEmpty()
				return td
			},
			err: errTooManyDeduplicationIds,
		},
	}

	t.Parallel()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			td, err := processor.ConsumeTraces(context.Background(), tc.generator())
			if tc.err == nil {
				assert.NoError(t, err)
				rss := td.ResourceSpans()
				for i := 0; i < rss.Len(); i++ {
					val, ok := rss.At(i).Resource().Attributes().Get(deduplicationIdKey)
					require.True(t, ok)
					require.Equal(t, tc.id, val.Str())
				}
			}

			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestConsumeLogs(t *testing.T) {
	processor := &deduplicationIdProcessor{
		IdGenerator: id.NewConstantGenerator("test"),
	}

	tests := []struct {
		name      string
		id        string
		generator func() plog.Logs
		err       error
	}{
		{
			name:      "LogsOneEmptyResourceLogs",
			id:        "test",
			generator: func() plog.Logs { return testdata.GenerateLogsOneEmptyResourceLogs() },
			err:       nil,
		},
		{
			name:      "LogsOneLogRecord",
			id:        "test",
			generator: func() plog.Logs { return testdata.GenerateLogsOneLogRecord() },
			err:       nil,
		},
		{
			name:      "LogsTwoLogRecordsSameResource",
			id:        "test",
			generator: func() plog.Logs { return testdata.GenerateLogsTwoLogRecordsSameResource() },
			err:       nil,
		},
		{
			name: "LogsOneSpanWithDeduplicationId",
			id:   "my-custom-id",
			generator: func() plog.Logs {
				td := testdata.GenerateLogsOneLogRecord()
				td.ResourceLogs().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: nil,
		},
		{
			name: "LogsTwoLogRecordsSameResourceOneDifferentWithDeduplicationId",
			id:   "my-custom-id",
			generator: func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResourceOneDifferent()
				td.ResourceLogs().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceLogs().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: nil,
		},
		{
			name: "GenerateLogsTwoLogRecordsSameResourceOneDifferentWithDifferentDeduplicationId",
			id:   "",
			generator: func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResourceOneDifferent()
				td.ResourceLogs().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceLogs().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id-2")
				return td
			},
			err: errTooManyDeduplicationIds,
		},
		{
			name: "LogsTwoLogRecordsSameResourceOneDifferentWithMissingDeduplicationId",
			id:   "",
			generator: func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResourceOneDifferent()
				td.ResourceLogs().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				return td
			},
			err: errMissingDepuplicationId,
		},
		{
			name: "GLogsTwoLogRecordsSameResourceOneDifferentWithDifferentAndMissingDeduplicationId",
			id:   "",
			generator: func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResourceOneDifferent()
				td.ResourceLogs().At(0).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id")
				td.ResourceLogs().At(1).Resource().Attributes().PutStr(deduplicationIdKey, "my-custom-id-2")
				td.ResourceLogs().AppendEmpty()
				return td
			},
			err: errTooManyDeduplicationIds,
		},
	}

	t.Parallel()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			td, err := processor.ConsumeLogs(context.Background(), tc.generator())
			if tc.err == nil {
				assert.NoError(t, err)
				rss := td.ResourceLogs()
				for i := 0; i < rss.Len(); i++ {
					val, ok := rss.At(i).Resource().Attributes().Get(deduplicationIdKey)
					require.True(t, ok)
					require.Equal(t, tc.id, val.Str())
				}
			}

			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
