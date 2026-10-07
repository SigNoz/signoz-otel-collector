package fieldvalues

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvaluestest"
)

const benchBatch = 1000

// countingWriter counts rows and keeps none, so a benchmark measures the
// writer and not the memory of the rows.
type countingWriter struct {
	mu   sync.Mutex
	rows int
}

func (w *countingWriter) write(_ context.Context, rows []row) error {
	w.mu.Lock()
	w.rows += len(rows)
	w.mu.Unlock()
	return nil
}

func newBenchWriter(b testing.TB, signal pipeline.Signal) (*Writer, *countingWriter) {
	b.Helper()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Cache.MaxBytes = 64 << 20
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), signal.String(), "")
	require.NoError(b, err)
	cw := &countingWriter{}
	w := newWriter(cfg, Settings{Signal: signal, Logger: zap.NewNop()}, cw, tel)
	w.allocate()
	w.now = func() time.Time { return testDay }
	return w, cw
}

// BenchmarkWriteLogsSteadyState writes batches whose sets and values the day
// cache mostly knows, as on a running collector.
func BenchmarkWriteLogsSteadyState(b *testing.B) {
	w, cw := newBenchWriter(b, pipeline.SignalLogs)
	gen := fieldvaluestest.NewGenerator(1, testDay)
	batches := make([]plog.Logs, 64)
	for i := range batches {
		batches[i] = gen.Logs(benchBatch)
	}
	for _, ld := range batches {
		_ = w.WriteLogs(context.Background(), ld)
	}
	cw.rows = 0
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = w.WriteLogs(context.Background(), batches[i%len(batches)])
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*benchBatch), "ns/record")
	b.ReportMetric(float64(cw.rows)/float64(b.N*benchBatch), "rows/record")
}

// BenchmarkWriteLogsNewData writes fresh batches: every batch has new user and
// request ids, and new sets until the resources reach their limits.
func BenchmarkWriteLogsNewData(b *testing.B) {
	w, cw := newBenchWriter(b, pipeline.SignalLogs)
	gen := fieldvaluestest.NewGenerator(2, testDay)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ld := gen.Logs(benchBatch)
		b.StartTimer()
		_ = w.WriteLogs(context.Background(), ld)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*benchBatch), "ns/record")
	b.ReportMetric(float64(cw.rows)/float64(b.N*benchBatch), "rows/record")
}

func BenchmarkWriteTracesNewData(b *testing.B) {
	w, cw := newBenchWriter(b, pipeline.SignalTraces)
	gen := fieldvaluestest.NewGenerator(3, testDay)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		td := gen.Traces(benchBatch)
		b.StartTimer()
		_ = w.WriteTraces(context.Background(), td)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*benchBatch), "ns/span")
	b.ReportMetric(float64(cw.rows)/float64(b.N*benchBatch), "rows/span")
}

func BenchmarkWriteMetricsNewData(b *testing.B) {
	w, cw := newBenchWriter(b, pipeline.SignalMetrics)
	gen := fieldvaluestest.NewGenerator(4, testDay)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		md := gen.Metrics(benchBatch)
		b.StartTimer()
		_ = w.WriteMetrics(context.Background(), md)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*benchBatch), "ns/point")
	b.ReportMetric(float64(cw.rows)/float64(b.N*benchBatch), "rows/point")
}

func BenchmarkWindowCache(b *testing.B) {
	c := newWindowCache(256<<20, 0.1)
	c.rotate(1)
	exact, _ := c.capacity()
	keys := make([]uint64, exact)
	for i := range keys {
		keys[i] = mix64(uint64(i) + 1)
	}
	b.Run("insert", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c.insert(keys[i%len(keys)], classExact)
		}
	})
	b.Run("has", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c.has(keys[i%len(keys)])
		}
	})
}

// Batches pushed at the same time must not race, and must not pass the room
// of the window cache together.
func TestConcurrentPushes(t *testing.T) {
	w, cw := newBenchWriter(t, pipeline.SignalLogs)
	w.state.cache.limit = [2]int{5000, 500}
	var batches []plog.Logs
	gen := fieldvaluestest.NewGenerator(5, testDay)
	for i := 0; i < 32; i++ {
		batches = append(batches, gen.Logs(200))
	}
	var wg sync.WaitGroup
	for _, ld := range batches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.WriteLogs(context.Background(), ld)
		}()
	}
	wg.Wait()
	assert.Positive(t, cw.rows)
	assert.LessOrEqual(t, w.state.cache.used[classExact], 5000)
	assert.LessOrEqual(t, w.state.cache.used[classReserve], 500)
	assert.Equal(t, [2]int{0, 0}, w.state.inflight)
	assert.Zero(t, w.state.inflightAhead)
}

// BenchmarkWriteCorrelatedNewData is BenchmarkWrite*NewData with correlated
// fields, closer to real data.
func BenchmarkWriteCorrelatedNewData(b *testing.B) {
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		b.Run(signal.String(), func(b *testing.B) {
			w, cw := newBenchWriter(b, signal)
			gen := fieldvaluestest.NewGenerator(6, testDay)
			gen.Correlated = true
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				var push func() error
				switch signal {
				case pipeline.SignalLogs:
					ld := gen.Logs(benchBatch)
					push = func() error { return w.WriteLogs(context.Background(), ld) }
				case pipeline.SignalTraces:
					td := gen.Traces(benchBatch)
					push = func() error { return w.WriteTraces(context.Background(), td) }
				default:
					md := gen.Metrics(benchBatch)
					push = func() error { return w.WriteMetrics(context.Background(), md) }
				}
				b.StartTimer()
				_ = push()
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*benchBatch), "ns/item")
			b.ReportMetric(float64(cw.rows)/float64(b.N*benchBatch), "rows/item")
		})
	}
}
