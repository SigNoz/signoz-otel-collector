package jsontypeexporter

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
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

	keyCache, err := lru.New[string, struct{}](100000)
	if err != nil {
		b.Fatalf("failed to create key cache: %v", err)
	}
	ld := buildLogs(10_000)

	// Construct exporter with key cache
	exp := &jsonTypeExporter{
		config:   &Config{},
		logger:   zap.NewNop(),
		limiter:  make(chan struct{}, utils.Concurrency()),
		conn:     conn,
		keyCache: keyCache,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exp.pushLogs(ctx, ld)
	}
}
