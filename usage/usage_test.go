package usage

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_windowStartOf(t *testing.T) {
	got := windowStartOf(time.Date(2026, 6, 30, 13, 42, 7, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 6, 30, 13, 0, 0, 0, time.UTC), got)

	// normalized to UTC: 15:30 +02:00 == 13:30 UTC -> window starts at 13:00 UTC
	plus2 := time.FixedZone("plus2", 2*60*60)
	assert.Equal(t, time.Date(2026, 6, 30, 13, 0, 0, 0, time.UTC),
		windowStartOf(time.Date(2026, 6, 30, 15, 30, 0, 0, plus2)))
}

func Test_windowExporterID(t *testing.T) {
	w := time.Date(2026, 6, 30, 13, 0, 0, 0, time.UTC)

	require.Equal(t, windowExporterID(w), windowExporterID(w))

	// same instant in another zone
	plus2 := time.FixedZone("plus2", 2*60*60)
	assert.Equal(t, windowExporterID(w), windowExporterID(w.In(plus2)))

	assert.NotEqual(t, windowExporterID(w), windowExporterID(w.Add(reducedUsageWindow)))
}

func Test_reducedWindowStarts(t *testing.T) {
	// settle = 15m, window = 1h, lookback = 3.
	// now = 13:20, readyUntil = 13:05, window containing readyUntil is [13:00,14:00),
	// so the newest fully-settled window is [12:00,13:00), then the two before it.
	now := time.Date(2026, 6, 30, 13, 20, 0, 0, time.UTC)
	got := reducedWindowStarts(now)

	require.Len(t, got, reducedUsageLookbackWindows)
	assert.Equal(t, time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC), got[0])
	assert.Equal(t, time.Date(2026, 6, 30, 11, 0, 0, 0, time.UTC), got[1])
	assert.Equal(t, time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC), got[2])

	horizon := now.Add(-reducedUsageSettleWindow)
	for _, ws := range got {
		assert.False(t, ws.Add(reducedUsageWindow).After(horizon), "window %s not settled", ws)
	}

	afterMidnight := time.Date(2026, 6, 30, 0, 20, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 6, 29, 23, 0, 0, 0, time.UTC), reducedWindowStarts(afterMidnight)[0])
}

func Test_reducedSampleCount(t *testing.T) {
	conn, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	require.NoError(t, err)
	conn.MatchExpectationsInOrder(false)

	cols := []cmock.ColumnType{{Name: "count()", Type: "UInt64"}}
	conn.ExpectQueryRow(`reduced_last_60s[\s\S]*GROUP BY[\s\S]*reduced_fingerprint`).
		WillReturnRow(cmock.NewRow(cols, []any{uint64(7)}))
	conn.ExpectQueryRow(`reduced_sum_60s[\s\S]*GROUP BY[\s\S]*reduced_fingerprint`).
		WillReturnRow(cmock.NewRow(cols, []any{uint64(5)}))

	uc := NewUsageCollector(
		uuid.New(),
		conn,
		Options{ReducedUsageTables: []string{
			"distributed_samples_v4_reduced_last_60s",
			"distributed_samples_v4_reduced_sum_60s",
		}},
		"signoz_metrics",
		nil,
		zap.NewNop(),
	)

	total, err := uc.reducedSampleCount(context.Background(), time.UnixMilli(0), time.UnixMilli(60000))
	require.NoError(t, err)
	assert.Equal(t, uint64(12), total)
	require.NoError(t, conn.ExpectationsWereMet())
}
