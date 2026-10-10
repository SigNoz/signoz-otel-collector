package fieldvalues

import (
	"context"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMemoryBytes(t *testing.T) {
	assert.Equal(t, uint64(1<<20), memoryBytes(CacheConfig{MaxBytes: 1 << 20}, 0), "a configured size is used as is")

	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	autoMemory.Lock()
	weights := autoMemory.weights
	autoMemory.weights = 5
	autoMemory.Unlock()
	t.Cleanup(func() {
		autoMemory.Lock()
		autoMemory.weights = weights
		autoMemory.Unlock()
	})
	debug.SetMemoryLimit(5 << 30)
	assert.Equal(t, uint64((5<<30)/10*3/5), memoryBytes(CacheConfig{}, 3), "logs, traces and metrics: metrics get 3 of 5 shares of 10% of the memory limit")
	assert.Equal(t, uint64((5<<30)/10/5), memoryBytes(CacheConfig{}, 1), "logs get 1 share")
	debug.SetMemoryLimit(256 << 20)
	assert.Equal(t, uint64(minMemoryBytes/5), memoryBytes(CacheConfig{}, 1), "at least 64 MiB for all writers")
}

func TestAutoMemoryCountsWritersWithoutASize(t *testing.T) {
	autoMemory.Lock()
	before := autoMemory.weights
	autoMemory.Unlock()
	cfg := testConfig()
	cfg.Cache.MaxBytes = 0
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), "logs", "")
	require.NoError(t, err)
	logs := newWriter(cfg, Settings{Signal: pipeline.SignalLogs, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	metrics := newWriter(cfg, Settings{Signal: pipeline.SignalMetrics, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	sized := newWriter(testConfig(), Settings{Signal: pipeline.SignalLogs, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	autoMemory.Lock()
	assert.Equal(t, before+1+3, autoMemory.weights, "a writer with max_bytes takes no share")
	autoMemory.Unlock()
	for _, w := range []*Writer{logs, metrics, sized, logs} {
		require.NoError(t, w.Shutdown())
	}
	autoMemory.Lock()
	assert.Equal(t, before, autoMemory.weights, "shutdown gives the share back once")
	autoMemory.Unlock()
}

func TestMemoryIsSplitBetweenCacheAndTracker(t *testing.T) {
	cfg := testConfig()
	cfg.Cache.MaxBytes = 256 << 20
	e, _ := newTestExporter(t, cfg, pipeline.SignalLogs)
	exact, reserve := e.state.cache.capacity()
	assert.Equal(t, bucketCapacity(96<<20), exact+reserve, "three quarters for the key cache: two buckets of 96 MiB, 80% full")
	assert.Equal(t, 64<<20, e.state.budget.limit, "a quarter for the tracker and the resource states")

	small, _ := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	exact, reserve = small.state.cache.capacity()
	assert.Equal(t, bucketCapacity(minBucketBytes), exact+reserve, "a bucket has at least the 32 MiB of pkg/timebucketedset")
}

func TestFailedInsertsWarnOncePerInterval(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), "logs", "")
	require.NoError(t, err)
	fw := &fakeWriter{fail: 3}
	w := newWriter(testConfig(), Settings{Signal: pipeline.SignalLogs, Logger: zap.New(core), Telemetry: componenttest.NewNopTelemetrySettings()}, fw, tel)
	w.now = func() time.Time { return testDay }
	for i := 0; i < 3; i++ {
		ld := logsOf(checkout, logRecord{"10:00", map[string]any{"n": int64(i)}})
		require.NoError(t, w.WriteLogs(context.Background(), ld), "a failed insert never fails the export")
	}
	assert.Equal(t, 1, logs.FilterMessage("failed to insert field values").Len())
}
