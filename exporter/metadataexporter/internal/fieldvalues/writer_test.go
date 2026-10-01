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

func TestCacheBytes(t *testing.T) {
	assert.Equal(t, uint64(1<<20), cacheBytes(CacheConfig{MaxBytes: 1 << 20}), "a configured size is used as is")

	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	debug.SetMemoryLimit(3 << 30)
	assert.Equal(t, uint64((3<<30)/10/3), cacheBytes(CacheConfig{}), "a third of 10% of the memory limit")
	debug.SetMemoryLimit(256 << 20)
	assert.Equal(t, uint64(minCacheBytes/3), cacheBytes(CacheConfig{}), "at least 64 MiB for the three signals")
}

func TestFailedInsertsWarnOncePerInterval(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), "logs", "")
	require.NoError(t, err)
	fw := &fakeWriter{fail: 3}
	w := newWriter(testConfig(), Settings{Signal: pipeline.SignalLogs, Logger: zap.New(core)}, fw, tel)
	w.now = func() time.Time { return testDay }
	for i := 0; i < 3; i++ {
		ld := logsOf(checkout, logRecord{"10:00", map[string]any{"n": int64(i)}})
		require.NoError(t, w.WriteLogs(context.Background(), ld), "a failed insert never fails the export")
	}
	assert.Equal(t, 1, logs.FilterMessage("failed to insert field values").Len())
}
