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

func Test_startOfUTCDay(t *testing.T) {
	got := startOfUTCDay(time.Date(2026, 6, 29, 13, 30, 45, 123, time.UTC))
	assert.Equal(t, time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC), got)
}

func Test_dayExporterID(t *testing.T) {
	day := time.Date(2026, 6, 29, 13, 0, 0, 0, time.UTC)

	// deterministic and identical across calls (i.e. across pods)
	require.Equal(t, dayExporterID(day), dayExporterID(day))

	// same id for any clock time within the same UTC day
	assert.Equal(t, dayExporterID(day), dayExporterID(time.Date(2026, 6, 29, 23, 59, 59, 0, time.UTC)))

	// normalized to UTC: a wall clock on the 30th in +02:00 that is still the
	// 29th in UTC maps to the 29th's id
	plus2 := time.FixedZone("plus2", 2*60*60)
	assert.Equal(t, dayExporterID(day), dayExporterID(time.Date(2026, 6, 30, 1, 30, 0, 0, plus2)))

	// a different UTC day gets a different id (its own epoch)
	assert.NotEqual(t, dayExporterID(day), dayExporterID(day.AddDate(0, 0, 1)))
}

func Test_reducedSampleCount(t *testing.T) {
	conn, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	require.NoError(t, err)
	conn.MatchExpectationsInOrder(false)

	cols := []cmock.ColumnType{{Name: "count()", Type: "UInt64"}}
	// one distinct-count query per configured reduced table; the results are summed.
	// the regex also asserts the distinct-by-reduced_fingerprint grouping shape.
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
