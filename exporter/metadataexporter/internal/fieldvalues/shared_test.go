package fieldvalues

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
)

// mapCache is a shared cache in memory, as two collectors would see Redis.
type mapCache struct {
	mu   sync.Mutex
	keys map[Window]map[uint64]struct{}
	err  error
}

func newMapCache() *mapCache { return &mapCache{keys: map[Window]map[uint64]struct{}{}} }

func (c *mapCache) Seen(_ context.Context, w Window, keys []uint64) ([]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	seen := make([]bool, len(keys))
	for i, k := range keys {
		_, seen[i] = c.keys[w][k]
	}
	return seen, nil
}

func (c *mapCache) Add(_ context.Context, w Window, keys []uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	if c.keys[w] == nil {
		c.keys[w] = map[uint64]struct{}{}
	}
	for _, k := range keys {
		c.keys[w][k] = struct{}{}
	}
	return nil
}

func (c *mapCache) Close() error { return nil }

func TestSharedCacheRemovesRepeatInsertsAcrossCollectors(t *testing.T) {
	shared := newMapCache()
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	collectorA, wa := newTestExporterWith(t, cfg, Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	collectorB, wb := newTestExporterWith(t, cfg, Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	ctx := context.Background()

	ld := logsOf(checkout,
		logRecord{"10:00", map[string]any{"http.method": "GET"}},
		logRecord{"10:01", map[string]any{"http.method": "POST"}},
	)
	require.NoError(t, collectorA.WriteLogs(ctx, ld))
	assert.Equal(t, []string{"{http.method=GET}", "{http.method=POST}"}, setList(wa.take()))

	require.NoError(t, collectorB.WriteLogs(ctx, ld))
	assert.Empty(t, wb.take(), "collector B finds every key in the shared cache")

	require.NoError(t, collectorB.WriteLogs(ctx, logsOf(checkout, logRecord{"10:02", map[string]any{"http.method": "PUT"}})))
	assert.Equal(t, []string{"{http.method=PUT}"}, setList(wb.take()), "a new set is still written")

	require.NoError(t, collectorB.WriteLogs(ctx, ld))
	assert.Empty(t, wb.take(), "the keys found in the shared cache are now in the local cache")
}

func TestSharedCacheErrorsNeverDropRows(t *testing.T) {
	shared := newMapCache()
	shared.err = errors.New("redis down")
	e, w := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	require.NoError(t, e.WriteLogs(context.Background(), logsOf(checkout, logRecord{"10:00", map[string]any{"http.method": "GET"}})))
	assert.Equal(t, []string{"{http.method=GET}"}, setList(w.take()), "a failed shared cache keeps every row")
}

func TestSharedCacheKeepsSamples(t *testing.T) {
	shared := newMapCache()
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 1
	a, wa := newTestExporterWith(t, cfg, Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	b, wb := newTestExporterWith(t, cfg, Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	ld := logsOf(checkout,
		logRecord{"10:00", map[string]any{"user.id": "u1"}},
		logRecord{"10:01", map[string]any{"user.id": "u2"}},
	)
	require.NoError(t, a.WriteLogs(context.Background(), ld))
	assert.Len(t, setList(wa.take()), 2)
	require.NoError(t, b.WriteLogs(context.Background(), ld))
	rows := wb.take()
	require.Len(t, rows, 1, "the sample value u2 has no cache key; each collector writes its own sample")
	assert.Equal(t, "user.id=u2", describe(rows[0].p))
}

func TestRedisCacheCommands(t *testing.T) {
	db, mock := redismock.NewClientMock()
	c := NewRedisCache(db, "tenant", "logs", "")
	t.Cleanup(func() { _ = c.Close() })
	day := Window{Start: 20718 * dayMillis, End: 20719 * dayMillis}
	k1, k2 := uint64(256*3+5), uint64(256*7+5)
	key := c.setKey(day, 5)

	mock.ExpectSMIsMember(key, member(k1), member(k2)).SetVal([]bool{true, false})
	seen, err := c.Seen(context.Background(), day, []uint64{k1, k2})
	require.NoError(t, err)
	assert.Equal(t, []bool{true, false}, seen, "both keys are in bucket 5")

	expireAt := time.Unix(20719*86400, 0).Add(redisKeepAfterWindow)
	mock.ExpectSAdd(key, member(k2)).SetVal(1)
	mock.ExpectExpireAt(key, expireAt).SetVal(true)
	require.NoError(t, c.Add(context.Background(), day, []uint64{k2}))
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, "tenant:field_values:logs::1790035200:5", key, "the set is named by the start of the window in seconds")
}

func TestRedisCacheErrorIsReturned(t *testing.T) {
	db, mock := redismock.NewClientMock()
	c := NewRedisCache(db, "tenant", "logs", "")
	t.Cleanup(func() { _ = c.Close() })
	w := Window{Start: 0, End: dayMillis}
	mock.ExpectSMIsMember(c.setKey(w, 1), member(1)).SetErr(errors.New("timeout"))
	_, err := c.Seen(context.Background(), w, []uint64{1})
	assert.Error(t, err)
}
