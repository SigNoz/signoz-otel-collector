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

func setAutoWriters(t *testing.T, n int) {
	t.Helper()
	autoMemory.Lock()
	writers := autoMemory.writers
	autoMemory.writers = n
	autoMemory.Unlock()
	t.Cleanup(func() {
		autoMemory.Lock()
		autoMemory.writers = writers
		autoMemory.Unlock()
	})
}

func setMemoryLimit(t *testing.T, limit int64) {
	t.Helper()
	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	debug.SetMemoryLimit(limit)
}

func TestMemoryBytes(t *testing.T) {
	assert.Equal(t, uint64(1<<20), memoryBytes(CacheConfig{MaxBytes: 1 << 20}), "a configured size is used as is")
	setAutoWriters(t, 4)
	setMemoryLimit(t, 3<<30)
	assert.Equal(t, uint64((3<<30)/10/4), memoryBytes(CacheConfig{}), "four writers share 10% of the memory limit")
	debug.SetMemoryLimit(256 << 20)
	assert.Equal(t, uint64(minMemoryBytes/4), memoryBytes(CacheConfig{}), "at least 64 MiB for all writers")
}

func TestAutoMemoryCountsWritersWithoutASize(t *testing.T) {
	autoMemory.Lock()
	before := autoMemory.writers
	autoMemory.Unlock()
	cfg := testConfig()
	cfg.Cache.MaxBytes = 0
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), "logs", "")
	require.NoError(t, err)
	a := newWriter(cfg, Settings{Signal: pipeline.SignalLogs, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	b := newWriter(cfg, Settings{Signal: pipeline.SignalMetrics, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	sized := newWriter(testConfig(), Settings{Signal: pipeline.SignalLogs, Logger: zap.NewNop()}, &fakeWriter{}, tel)
	autoMemory.Lock()
	assert.Equal(t, before+2, autoMemory.writers, "a writer with max_bytes takes no share")
	autoMemory.Unlock()
	for _, w := range []*Writer{a, b, sized, a} {
		require.NoError(t, w.Shutdown())
	}
	autoMemory.Lock()
	assert.Equal(t, before, autoMemory.writers, "shutdown gives the share back once")
	autoMemory.Unlock()
}

// Writers without max_bytes keep half of their key cache as a floor and share
// the rest: the writer that meets the most sets gets the most keys.
func TestAutoWritersShareTheirKeys(t *testing.T) {
	setAutoWriters(t, 0)
	setMemoryLimit(t, 4<<30)
	cfg := testConfig()
	cfg.Cache.MaxBytes = 0
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), "logs", "")
	require.NoError(t, err)
	var writers []*Writer
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		writers = append(writers, newWriter(cfg, Settings{Signal: signal, Logger: zap.NewNop(), Telemetry: componenttest.NewNopTelemetrySettings()}, &fakeWriter{}, tel))
	}
	for _, w := range writers {
		require.NoError(t, w.allocate())
	}
	t.Cleanup(func() {
		for _, w := range writers {
			_ = w.Shutdown()
		}
	})
	total := autoTotal()
	pool := windowKeys(total / 8 * 3)
	floor := windowKeys(total / 3 / 4 * 3 / 2)
	logs, metrics := writers[0].state.cache, writers[2].state.cache
	assert.Equal(t, int64(pool), autoPool.size.Load(), "the pool is half of the key cache of all writers")

	logs.used[classExact] = logs.floor + pool
	logs.exactUsed.Store(int64(logs.used[classExact]))
	assert.False(t, logs.room(classExact, 1), "logs took their floor and the whole pool")
	assert.True(t, metrics.room(classExact, metrics.floor), "metrics keep their floor")
	assert.False(t, metrics.room(classExact, metrics.floor+1), "but get nothing from the pool")
	assert.InDelta(t, floor, metrics.floor+metrics.limit[classReserve], 1)

	logs.rotate(logs.window + logs.width + 1)
	assert.True(t, metrics.room(classExact, metrics.floor+pool), "a new window gives the pool back")
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
