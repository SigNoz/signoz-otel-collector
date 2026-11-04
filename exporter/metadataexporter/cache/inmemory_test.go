package metadataexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func TestInMemoryKeyCache_BasicAddGet(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              10,
		MaxMetricsResourceFp:             10,
		MaxLogsResourceFp:                10,
		MaxTracesCardinalityPerResource:  5,
		MaxMetricsCardinalityPerResource: 5,
		MaxLogsCardinalityPerResource:    5,
		TracesFingerprintCacheTTL:        2 * time.Second,
		MetricsFingerprintCacheTTL:       2 * time.Second,
		LogsFingerprintCacheTTL:          2 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}
	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)
	require.NotNil(t, cache)

	resourceFp := uint64(1111)
	attrFps := []uint64{555, 666, 777}

	// Add to TRACES
	err = cache.AddAttrsToResource(ctx, resourceFp, attrFps, pipeline.SignalTraces)
	require.NoError(t, err, "should add attributes without error")

	// Check existence
	exists, err := cache.AttrsExistForResource(ctx, resourceFp, attrFps, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true, true}, exists, "all should exist")

	// For a resource that doesn't exist:
	existsNon, err := cache.AttrsExistForResource(ctx, 9999, attrFps, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{false, false, false}, existsNon, "none should exist")
}

func TestInMemoryKeyCache_ResourceLimit(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              2, // Only 2 resources allowed
		MaxTracesCardinalityPerResource:  5,
		TracesFingerprintCacheTTL:        10 * time.Second,
		MaxMetricsResourceFp:             10,
		MaxMetricsCardinalityPerResource: 10,
		MetricsFingerprintCacheTTL:       10 * time.Second,
		MaxLogsResourceFp:                10,
		MaxLogsCardinalityPerResource:    10,
		LogsFingerprintCacheTTL:          10 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	// Add resource #1
	err = cache.AddAttrsToResource(ctx, 100, []uint64{1, 2}, pipeline.SignalTraces)
	require.NoError(t, err)

	// Add resource #2
	err = cache.AddAttrsToResource(ctx, 101, []uint64{3}, pipeline.SignalTraces)
	require.NoError(t, err)

	// Resource #3 -> should exceed resource limit
	err = cache.AddAttrsToResource(ctx, 102, []uint64{4}, pipeline.SignalTraces)
	require.Error(t, err, "should fail because only 2 resources are allowed in traces")

	// Confirm the first 2 resources are still valid
	exists, err := cache.AttrsExistForResource(ctx, 100, []uint64{1}, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{true}, exists)
}

func TestInMemoryKeyCache_AttrCardinalityLimit(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              10,
		MaxTracesCardinalityPerResource:  3, // only 3 attributes per resource
		TracesFingerprintCacheTTL:        10 * time.Second,
		MaxMetricsResourceFp:             10,
		MaxMetricsCardinalityPerResource: 10,
		MetricsFingerprintCacheTTL:       10 * time.Second,
		MaxLogsResourceFp:                10,
		MaxLogsCardinalityPerResource:    10,
		LogsFingerprintCacheTTL:          10 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	resourceFp := uint64(999)
	// Add 2 attributes
	err = cache.AddAttrsToResource(ctx, resourceFp, []uint64{1, 2}, pipeline.SignalTraces)
	require.NoError(t, err)

	// Add 1 more
	err = cache.AddAttrsToResource(ctx, resourceFp, []uint64{3}, pipeline.SignalTraces)
	require.NoError(t, err)

	// Try to add 4th
	err = cache.AddAttrsToResource(ctx, resourceFp, []uint64{4}, pipeline.SignalTraces)
	require.Error(t, err, "exceeding the cardinality limit per resource")

	exists, err := cache.AttrsExistForResource(ctx, resourceFp, []uint64{1, 2, 3, 4}, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true, true, false}, exists)
}

func TestInMemoryKeyCache_TTLExpiration(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              10,
		MaxTracesCardinalityPerResource:  10,
		TracesFingerprintCacheTTL:        500 * time.Millisecond, // short TTL
		MaxMetricsResourceFp:             10,
		MaxMetricsCardinalityPerResource: 10,
		MetricsFingerprintCacheTTL:       10 * time.Second,
		MaxLogsResourceFp:                10,
		MaxLogsCardinalityPerResource:    10,
		LogsFingerprintCacheTTL:          10 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	resourceFp := uint64(1234)
	attrFps := []uint64{1, 2}

	err = cache.AddAttrsToResource(ctx, resourceFp, attrFps, pipeline.SignalTraces)
	require.NoError(t, err)

	exists, err := cache.AttrsExistForResource(ctx, resourceFp, attrFps, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true}, exists, "they exist initially")

	// Sleep to exceed TTL
	time.Sleep(600 * time.Millisecond)

	// The tracesCache entry should have been evicted
	existsAfter, err := cache.AttrsExistForResource(ctx, resourceFp, attrFps, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{false, false}, existsAfter, "entry should have expired")
}

func TestInMemoryKeyCache_EmptyAttrSlice(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              5,
		MaxTracesCardinalityPerResource:  5,
		TracesFingerprintCacheTTL:        2 * time.Second,
		MaxMetricsResourceFp:             5,
		MaxMetricsCardinalityPerResource: 5,
		MetricsFingerprintCacheTTL:       2 * time.Second,
		MaxLogsResourceFp:                5,
		MaxLogsCardinalityPerResource:    5,
		LogsFingerprintCacheTTL:          2 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}
	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	// Adding no attributes should not fail
	err = cache.AddAttrsToResource(ctx, 111, []uint64{}, pipeline.SignalTraces)
	assert.NoError(t, err)

	// Checking existence for empty slice should return empty slice
	exists, err := cache.AttrsExistForResource(ctx, 111, []uint64{}, pipeline.SignalTraces)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(exists))
}

func TestInMemoryKeyCache_TotalCardinalityLimit(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              10,
		MaxTracesCardinalityPerResource:  5,
		TracesMaxTotalCardinality:        10,
		TracesFingerprintCacheTTL:        2 * time.Second,
		MaxMetricsResourceFp:             10,
		MaxMetricsCardinalityPerResource: 5,
		MetricsMaxTotalCardinality:       10,
		MetricsFingerprintCacheTTL:       2 * time.Second,
		MaxLogsResourceFp:                10,
		MaxLogsCardinalityPerResource:    5,
		LogsMaxTotalCardinality:          10,
		LogsFingerprintCacheTTL:          2 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	_ = cache.AddAttrsToResource(ctx, 111, []uint64{1, 2}, pipeline.SignalTraces)
	_ = cache.AddAttrsToResource(ctx, 112, []uint64{3, 4}, pipeline.SignalTraces)
	_ = cache.AddAttrsToResource(ctx, 113, []uint64{5, 6}, pipeline.SignalTraces)
	_ = cache.AddAttrsToResource(ctx, 114, []uint64{7, 8}, pipeline.SignalTraces)
	_ = cache.AddAttrsToResource(ctx, 115, []uint64{9, 10}, pipeline.SignalTraces)

	exists, err := cache.AttrsExistForResource(ctx, 111, []uint64{1, 2}, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true}, exists)

	exceeds := cache.TotalCardinalityLimitExceeded(ctx, pipeline.SignalTraces)
	assert.True(t, exceeds, "total cardinality limit should be exceeded")
}

func TestInMemoryKeyCache_NonExistentResource(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              5,
		MaxTracesCardinalityPerResource:  5,
		TracesFingerprintCacheTTL:        2 * time.Second,
		MaxMetricsResourceFp:             5,
		MaxMetricsCardinalityPerResource: 5,
		MetricsFingerprintCacheTTL:       2 * time.Second,
		MaxLogsResourceFp:                5,
		MaxLogsCardinalityPerResource:    5,
		LogsFingerprintCacheTTL:          2 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}
	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	exists, err := cache.AttrsExistForResource(ctx, 9999, []uint64{1, 2}, pipeline.SignalTraces)
	require.NoError(t, err)
	assert.Equal(t, []bool{false, false}, exists, "non-existent resource => false for all")
}

func TestInMemoryKeyCache_Debug(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:              10,
		MaxTracesCardinalityPerResource:  10,
		TracesFingerprintCacheTTL:        2 * time.Second,
		MaxMetricsResourceFp:             10,
		MaxMetricsCardinalityPerResource: 10,
		MetricsFingerprintCacheTTL:       2 * time.Second,
		MaxLogsResourceFp:                10,
		MaxLogsCardinalityPerResource:    10,
		LogsFingerprintCacheTTL:          2 * time.Second,
		TenantID:                         "tenant1",
		Logger:                           logger,
	}
	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	// Add some resources
	_ = cache.AddAttrsToResource(ctx, 111, []uint64{1, 2, 3}, pipeline.SignalTraces)
	_ = cache.AddAttrsToResource(ctx, 222, []uint64{99}, pipeline.SignalMetrics)

	// Just verify we don't panic or error
	cache.Debug(ctx)
}

func TestInMemoryKeyCache_SignalLimitsIsolation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		// Different resource capacities for each signal:
		MaxTracesResourceFp:  2,
		MaxMetricsResourceFp: 2,
		MaxLogsResourceFp:    3,

		// Different per-resource cardinality capacities:
		MaxTracesCardinalityPerResource:  5,
		MaxMetricsCardinalityPerResource: 10,
		MaxLogsCardinalityPerResource:    15,

		// Different total cardinality limits:
		TracesMaxTotalCardinality:  10,
		MetricsMaxTotalCardinality: 20,
		LogsMaxTotalCardinality:    30,

		TracesFingerprintCacheTTL:  10 * time.Second,
		MetricsFingerprintCacheTTL: 10 * time.Second,
		LogsFingerprintCacheTTL:    10 * time.Second,
		TenantID:                   "tenant1",
		Logger:                     logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	// --- 1. Test Resource Limits Isolation ---
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalTraces))
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalMetrics))
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalLogs))

	// Traces capacity = 2
	_ = cache.AddAttrsToResource(ctx, 101, []uint64{1}, pipeline.SignalTraces)
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalTraces), "traces should not be full yet")
	
	_ = cache.AddAttrsToResource(ctx, 102, []uint64{1}, pipeline.SignalTraces)
	assert.True(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalTraces), "traces should be full")
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalMetrics), "metrics should not be full")
	assert.False(t, cache.ResourcesLimitExceeded(ctx, pipeline.SignalLogs), "logs should not be full")

	// --- 2. Test Per-Resource Cardinality Limits Isolation ---
	// Traces max attr limit = 5, Metrics max attr limit = 10
	err = cache.AddAttrsToResource(ctx, 101, []uint64{2, 3, 4, 5, 6}, pipeline.SignalTraces)
	assert.Error(t, err, "should exceed traces limit of 5 attributes")

	err = cache.AddAttrsToResource(ctx, 201, []uint64{1, 2, 3, 4, 5, 6, 7}, pipeline.SignalMetrics)
	assert.NoError(t, err, "should not exceed metrics limit of 10 attributes")

	// --- 3. Test Total Cardinality Limit Isolation ---
	// Traces total cap = 10, Metrics total cap = 20
	// Currently, resource 101 has 1 attribute, resource 102 has 1 attribute.
	assert.False(t, cache.TotalCardinalityLimitExceeded(ctx, pipeline.SignalTraces))

	// Add 4 more attributes to 101 (total = 5, which is the per-resource limit)
	_ = cache.AddAttrsToResource(ctx, 101, []uint64{2, 3, 4, 5}, pipeline.SignalTraces)

	// Add 4 more attributes to 102 (total = 5, which is the per-resource limit)
	_ = cache.AddAttrsToResource(ctx, 102, []uint64{10, 11, 12, 13}, pipeline.SignalTraces)

	assert.True(t, cache.TotalCardinalityLimitExceeded(ctx, pipeline.SignalTraces), "traces total cardinality should be exceeded")
	assert.False(t, cache.TotalCardinalityLimitExceeded(ctx, pipeline.SignalMetrics), "metrics total cardinality should not be exceeded")
}

func TestInMemoryKeyCache_ConcurrentEvictionNoPanic(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	opts := InMemoryKeyCacheOptions{
		MaxTracesResourceFp:             100,
		MaxTracesCardinalityPerResource: 5,
		TracesFingerprintCacheTTL:       5 * time.Millisecond, // very short TTL
		TenantID:                        "tenant1",
		Logger:                          logger,
	}

	cache, err := NewInMemoryKeyCache(opts)
	require.NoError(t, err)

	stopChan := make(chan struct{})

	// Goroutine 1: Rapidly set entries to expire
	go func() {
		var i uint64
		for {
			select {
			case <-stopChan:
				return
			default:
				_ = cache.AddAttrsToResource(ctx, i, []uint64{1}, pipeline.SignalTraces)
				i = (i + 1) % 100
				time.Sleep(50 * time.Microsecond)
			}
		}
	}()

	// Goroutine 2: Constantly query TotalCardinalityLimitExceeded
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				_ = cache.TotalCardinalityLimitExceeded(ctx, pipeline.SignalTraces)
				time.Sleep(50 * time.Microsecond)
			}
		}
	}()

	// Run for 500ms to verify no panics occur
	time.Sleep(500 * time.Millisecond)
	close(stopChan)
}
