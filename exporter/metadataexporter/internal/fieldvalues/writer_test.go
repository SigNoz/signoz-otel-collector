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
	assert.Equal(t, uint64(1<<20), memoryBytes(CacheConfig{MaxBytes: 1 << 20}), "a configured size is used as is")

	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	autoMemory.Lock()
	writers := autoMemory.writers
	autoMemory.writers = 4
	autoMemory.Unlock()
	t.Cleanup(func() {
		autoMemory.Lock()
		autoMemory.writers = writers
		autoMemory.Unlock()
	})
	debug.SetMemoryLimit(3 << 30)
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
	b := newWriter(cfg, Settings{Signal: pipeline.SignalLogs, Logger: zap.NewNop()}, &fakeWriter{}, tel)
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
