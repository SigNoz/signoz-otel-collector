package metadataexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// Regression coverage for the signal-specific total limits used by the collector.
// See https://github.com/SigNoz/signoz/issues/12596.
func TestInMemoryKeyCache_TotalCardinalityBySignal(t *testing.T) {
	signals := []pipeline.Signal{pipeline.SignalTraces, pipeline.SignalMetrics, pipeline.SignalLogs}
	for _, signal := range signals {
		t.Run(signal.String(), func(t *testing.T) {
			cache, err := NewInMemoryKeyCache(InMemoryKeyCacheOptions{
				MaxTracesResourceFp: 10, MaxMetricsResourceFp: 10, MaxLogsResourceFp: 10,
				MaxTracesCardinalityPerResource: 2, MaxMetricsCardinalityPerResource: 2, MaxLogsCardinalityPerResource: 2,
				TracesMaxTotalCardinality: 3, MetricsMaxTotalCardinality: 3, LogsMaxTotalCardinality: 3,
				TracesFingerprintCacheTTL: time.Hour, MetricsFingerprintCacheTTL: time.Hour, LogsFingerprintCacheTTL: time.Hour,
				Logger: zap.NewNop(),
			})
			require.NoError(t, err)
			t.Cleanup(func() { cache.tracesCache.Stop(); cache.metricsCache.Stop(); cache.logsCache.Stop() })
			ctx := context.Background()
			// Other signals already exceed their totals and must not affect this one.
			for _, other := range signals {
				if other != signal {
					require.NoError(t, cache.AddAttrsToResource(ctx, 1, []uint64{1, 2}, other))
					require.NoError(t, cache.AddAttrsToResource(ctx, 2, []uint64{3, 4}, other))
				}
			}
			require.NoError(t, cache.AddAttrsToResource(ctx, 1, []uint64{1, 2}, signal))
			require.False(t, cache.TotalCardinalityLimitExceeded(ctx, signal), "a full resource is still below the total limit")
			require.NoError(t, cache.AddAttrsToResource(ctx, 2, []uint64{3}, signal))
			require.True(t, cache.TotalCardinalityLimitExceeded(ctx, signal), "attributes across resources reach the total limit")
		})
	}
}
