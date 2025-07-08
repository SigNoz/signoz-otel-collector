package metadataexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// A helper to build the RedisKeyCache with a mocked client.
func buildMockRedisKeyCache(_ *testing.T, mockFn func(redismock.ClientMock)) (*RedisKeyCache, redismock.ClientMock) {
	db, mock := redismock.NewClientMock()
	// The caller can define mock expectations
	if mockFn != nil {
		mockFn(mock)
	}

	cache := &RedisKeyCache{
		redisClient: db,
		tenantID:    "testTenant",
		logger:      zap.NewNop(),

		tracesTTL:  10 * time.Second,
		metricsTTL: 20 * time.Second,
		logsTTL:    30 * time.Second,

		maxTracesResourceFp:              2,
		maxMetricsResourceFp:             5,
		maxLogsResourceFp:                5,
		maxTracesCardinalityPerResource:  3,
		maxMetricsCardinalityPerResource: 10,
		maxLogsCardinalityPerResource:    10,
		tracesMaxTotalCardinality:        20,
		metricsMaxTotalCardinality:       20,
		logsMaxTotalCardinality:          20,
	}
	return cache, mock
}

func TestRedisKeyCache_AddAttrsToResource_NewResource_Success(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	resourceHLLKey := fmt.Sprintf("testTenant:metadata:traces:%d:resources:hll", epochWindow)
	attrsKey := fmt.Sprintf("testTenant:metadata:traces:%d:resource:1000", epochWindow)
	attrsHLLKey := fmt.Sprintf("testTenant:metadata:traces:%d:attrs:hll", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// 1) First check if resource exists
		m.ExpectPFCount(resourceHLLKey).SetVal(0)

		// 3) Then check existing attributes count
		m.ExpectSCard(attrsKey).SetVal(0)

		// 2) Then check resource count
		m.ExpectPFAdd(resourceHLLKey, "1000").SetVal(0)
		m.ExpectExpire(resourceHLLKey, 10*time.Second).SetVal(true)

		m.ExpectSAdd(attrsKey, "1", "2").SetVal(2)
		m.ExpectExpire(attrsKey, 10*time.Second).SetVal(true)

		m.ExpectPFAdd(attrsHLLKey, "1", "2").SetVal(0)
		m.ExpectExpire(attrsHLLKey, 10*time.Second).SetVal(true)
	})

	err := cache.AddAttrsToResource(ctx, 1000, []uint64{1, 2}, pipeline.SignalTraces)
	require.NoError(t, err)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisKeyCache_AddAttrsToResource_ResourceLimitExceeded(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	resourceHLLKey := fmt.Sprintf("testTenant:metadata:traces:%d:resources:hll", epochWindow)
	attrsKey := fmt.Sprintf("testTenant:metadata:traces:%d:resource:1000", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// 1) SIsMember => false (resource not exist)
		m.ExpectPFCount(resourceHLLKey).SetVal(2)
		// 2) SCard => 2 (already 2 resources exist)
		m.ExpectSCard(attrsKey).SetVal(2)
	})

	err := cache.AddAttrsToResource(ctx, 2000, []uint64{123}, pipeline.SignalTraces)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many resource fingerprints")
	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_AddAttrsToResource_AttrCardinalityExceeded(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	resourceHLLKey := fmt.Sprintf("testTenant:metadata:traces:%d:resources:hll", epochWindow)
	attrsKey := fmt.Sprintf("testTenant:metadata:traces:%d:resource:3000", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// For an existing resource:
		m.ExpectPFCount(resourceHLLKey).SetVal(1)
		// Then we do not check resource set cardinality
		// Next step is SCard on resource:3000
		m.ExpectSCard(attrsKey).SetVal(3)
	})

	err := cache.AddAttrsToResource(ctx, 3000, []uint64{99}, pipeline.SignalTraces)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many attribute fingerprints")
	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_AddAttrsToResource_EmptyList(t *testing.T) {
	ctx := context.Background()

	cache, mock := buildMockRedisKeyCache(t, nil) // no mocks needed if we skip commands

	// Adding empty list of attributes should do nothing
	err := cache.AddAttrsToResource(ctx, 9999, nil, pipeline.SignalTraces)
	require.NoError(t, err)

	// Or we can pass an empty slice:
	err = cache.AddAttrsToResource(ctx, 9999, []uint64{}, pipeline.SignalTraces)
	require.NoError(t, err)

	// No Redis calls expected
	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_AttrsExistForResource_Basic(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	attrsKey := fmt.Sprintf("testTenant:metadata:metrics:%d:resource:5555", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// We expect an SMIsMember call
		members := []interface{}{"10", "20", "30"}
		m.ExpectSMIsMember(attrsKey, members...).
			SetVal([]bool{true, false, true})
	})

	exists, err := cache.AttrsExistForResource(ctx, 5555, []uint64{10, 20, 30}, pipeline.SignalMetrics)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, false, true}, exists)

	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_AttrsExistForResource_Empty(t *testing.T) {
	ctx := context.Background()
	cache, mock := buildMockRedisKeyCache(t, nil)

	exists, err := cache.AttrsExistForResource(ctx, 1234, []uint64{}, pipeline.SignalLogs)
	require.NoError(t, err)
	assert.Nil(t, exists) // or an empty slice

	// No calls
	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_ResourcesLimitExceeded(t *testing.T) {
	ctx := context.Background()

	epochWindow := getCurrentEpochWindowMillis()
	resourceHLLKey := fmt.Sprintf("testTenant:metadata:traces:%d:resources:hll", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		m.ExpectPFCount(resourceHLLKey).SetVal(3)
	})

	// The function under test:
	limitExceeded := cache.ResourcesLimitExceeded(ctx, pipeline.SignalTraces)
	assert.True(t, limitExceeded)

	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_CardinalityLimitExceeded(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	attrsKey := fmt.Sprintf("testTenant:metadata:traces:%d:resource:777", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// SCard => 3 for resource=777 => limit is 3 for traces
		m.ExpectSCard(attrsKey).SetVal(3)
	})

	exceeded := cache.CardinalityLimitExceeded(ctx, 777, pipeline.SignalTraces)
	assert.True(t, exceeded)

	_ = mock.ExpectationsWereMet()
}

func TestRedisKeyCache_Debug(t *testing.T) {
	ctx := context.Background()
	epochWindow := getCurrentEpochWindowMillis()
	resourceSetKey := fmt.Sprintf("testTenant:metadata:traces:%d:resources", epochWindow)
	attrsKey := fmt.Sprintf("testTenant:metadata:traces:%d:resource:1000", epochWindow)

	cache, mock := buildMockRedisKeyCache(t, func(m redismock.ClientMock) {
		// For Debug, we do a KEYS call
		m.ExpectKeys("testTenant:metadata:*").SetVal([]string{
			resourceSetKey,
			attrsKey,
		})
		m.ExpectSCard(resourceSetKey).SetVal(1)
		m.ExpectSCard(attrsKey).SetVal(2)
	})

	cache.Debug(ctx)
	_ = mock.ExpectationsWereMet()
}
