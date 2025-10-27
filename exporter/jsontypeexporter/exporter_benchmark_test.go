package jsontypeexporter

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

// buildLogs constructs a plog.Logs with count log records, using plogsgen for bodies.
func buildLogs(count int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lrs := sl.LogRecords()
	lrs.EnsureCapacity(count)

	gen := plogsgen.NewDataGenerator(count, 32)
	batch, _ := gen.GenerateBatch()
	for i := 0; i < count; i++ {
		lr := lrs.AppendEmpty()
		lr.SetTimestamp(pcommon.Timestamp(0))
		lr.Body().FromRaw(batch.Rows[i].Body)
	}
	return ld
}

func BenchmarkPushLogs_10k(b *testing.B) {
	ctx := context.Background()

	// Construct exporter directly (avoid external ClickHouse dependency in benchmark)
	exp := &jsonTypeExporter{
		config:  &Config{},
		logger:  zap.NewNop(),
		limiter: make(chan struct{}, utils.Concurrency()),
	}

	ld := buildLogs(10_000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exp.pushLogs(ctx, ld)
	}
}
