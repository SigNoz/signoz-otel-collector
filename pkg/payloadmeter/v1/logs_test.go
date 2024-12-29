package v1

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/payloadmeter"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"testing"

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

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	meter, err := NewLogsMeter(logger, payloadmeter.Json)
	if err != nil {
		panic(err)
	}
	size := meter.Size(logs, payloadmeter.Signoz)
	// 8 * [ 10(key) + 20(value) + 5("":"") ] + 2({}) + 7(,)
	assert.Equal(t, 10*(8*(10+20+5)+7+2+2+100), size)
}

func benchmarkLogsSize(b *testing.B, expectedSize int, options ...plogsgen.GenerationOption) {
	b.Helper()

	logs := plogsgen.Generate(options...)
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	meter, err := NewLogsMeter(logger, payloadmeter.Json)
	if err != nil {
		panic(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		size := meter.Size(logs, payloadmeter.Signoz)
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
