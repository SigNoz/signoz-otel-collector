package jsontypeexporter

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	mockhouse "github.com/srikanthccv/ClickHouse-go-mock"
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
	conn, err := mockhouse.NewClickHouseNative(nil)
	if err != nil {
		b.Fatalf("failed to create mock house: %v", err)
	}

	ld := buildLogs(10_000)

	// Construct exporter with key cache
	exp, err := setupTestExporter()
	if err != nil {
		b.Fatalf("failed to create test exporter: %v", err)
	}
	exp.conn = conn
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exp.pushLogs(ctx, ld)
	}
}
