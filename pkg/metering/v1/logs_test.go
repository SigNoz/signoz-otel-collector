package v1

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/stretchr/testify/assert"
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
