package v1

import (
	"strings"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestLogsSize(t *testing.T) {
	logs := plogsgen.Generate(
		plogsgen.WithLogRecordCount(10),
		plogsgen.WithResourceAttributeCount(8),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)

	meter := NewLogs(zap.NewNop())
	size := meter.Size(logs)
	// 8 * [ 10(key) + 20(value) + 5("":"") ] + 2({}) + 7(,)
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100), size)
}

func TestLogsSizeExcludesUncountedFields(t *testing.T) {
	logs := plogsgen.Generate(
		plogsgen.WithLogRecordCount(10),
		plogsgen.WithResourceAttributeCount(8),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)
	large := strings.Repeat("x", 10000)
	resourceLogs := logs.ResourceLogs().At(0)
	resourceLogs.SetSchemaUrl(large)
	resourceLogs.Resource().SetDroppedAttributesCount(7)
	scopeLogs := resourceLogs.ScopeLogs().At(0)
	scopeLogs.SetSchemaUrl(large)
	scopeLogs.Scope().SetName(large)
	scopeLogs.Scope().SetVersion(large)
	scopeLogs.Scope().Attributes().PutStr("scope.attribute", large)
	scopeLogs.Scope().SetDroppedAttributesCount(7)
	records := scopeLogs.LogRecords()
	for i := 0; i < records.Len(); i++ {
		record := records.At(i)
		record.SetSeverityText(large)
		record.SetSeverityNumber(plog.SeverityNumberFatal4)
		record.SetEventName(large)
		record.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		record.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
		record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
		record.SetDroppedAttributesCount(7)
	}

	meter := NewLogs(zap.NewNop())
	// Same as TestLogsSize: only body, log attributes and resource attributes count.
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100), meter.Size(logs))
}

func TestLogsSizeWithExcludedSigNozResources(t *testing.T) {
	logs := plogsgen.Generate(
		plogsgen.WithLogRecordCount(10),
		plogsgen.WithResourceAttributeCount(8),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)
	// adding signoz resource shouldn't affect the calculation
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("signoz.workspace.internal.test", "signoz-test")

	meter := NewLogs(zap.NewNop())
	size := meter.Size(logs)
	// 8 * [ 10(key) + 20(value) + 5("":"") ] + 2({}) + 7(,)
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100), size)
}

func TestLogsSizeWithExcludedSigNozInternalAttrs(t *testing.T) {
	logs := plogsgen.Generate(
		plogsgen.WithLogRecordCount(10),
		plogsgen.WithResourceAttributeCount(8),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)
	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	record.Attributes().PutStr(constants.OriginalBodyAttributeKey, `{"msg":"Lorem ipsum dolor sit amet consectetur adipiscing elit"}`)

	meter := NewLogs(zap.NewNop())
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100), meter.Size(logs))

	record.Attributes().PutStr("k", "v")
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100)+7, meter.Size(logs))
}

func benchmarkLogsSize(b *testing.B, expectedSize int, options ...plogsgen.GenerationOption) {
	b.Helper()

	logs := plogsgen.Generate(options...)
	meter := NewLogs(zap.NewNop())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		size := meter.Size(logs)
		assert.Equal(b, expectedSize, size)
	}
}

func BenchmarkLogsSize_20000_20(b *testing.B) {
	benchmarkLogsSize(
		b,
		16660000,
		plogsgen.WithLogRecordCount(20000),
		plogsgen.WithResourceAttributeCount(20),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)
}

func BenchmarkLogsSize_100000_20(b *testing.B) {
	benchmarkLogsSize(
		b,
		83300000,
		plogsgen.WithLogRecordCount(100000),
		plogsgen.WithResourceAttributeCount(20),
		// 100 bytes
		plogsgen.WithBody("Lorem ipsum dolor sit amet consectetur adipiscing elit, enim suscipit nullam aenean mattis senectus."),
		// 20 bytes
		plogsgen.WithResourceAttributeStringValue("Lorem ipsum euismod."),
	)
}
