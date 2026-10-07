package fieldvalues

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
)

var (
	midnight = testDay.Truncate(24 * time.Hour).Add(24 * time.Hour)
	// lastMillisecond is the end of the pre-write window of testDay, when
	// every key is due.
	lastMillisecond = midnight.Add(-time.Millisecond)
)

func logsAt(ts time.Time, resource map[string]any, attrs map[string]any) plog.Logs {
	ld := logsOf(resource, logRecord{"00:00", attrs})
	ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetTimestamp(pcommon.NewTimestampFromTime(ts))
	return ld
}

func millis(t time.Time) uint64 {
	return uint64(t.UnixMilli())
}

func seenTimes(rows []row) map[uint64]int {
	out := map[uint64]int{}
	for _, r := range rows {
		out[r.seenMillis]++
	}
	return out
}

// A record of 23:59 that arrives after midnight is written in the new window,
// at its start. Before, it kept its time: the set was cached for the new day
// with rows of the day before, and wrote nothing more all day.
func TestLateRecordIsStampedInItsWindow(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	late := logsAt(midnight.Add(-time.Minute), checkout, map[string]any{"http.method": "GET"})

	setNow(e, midnight.Add(5*time.Minute))
	require.NoError(t, e.WriteLogs(ctx, late))
	rows := w.take()
	assert.Len(t, rows, 3, "the set and the two resource fields")
	assert.Equal(t, map[uint64]int{millis(midnight): 3}, seenTimes(rows))

	setNow(e, midnight.Add(10*time.Hour))
	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(10*time.Hour), checkout, map[string]any{"http.method": "GET"})))
	assert.Empty(t, w.take(), "the rows of today exist")
}

func TestMetricsLateRecordIsStampedInItsWindow(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	setNow(e, midnight.Add(5*time.Minute))
	require.NoError(t, e.WriteMetrics(context.Background(), testMetrics()))
	rows := w.take()
	require.NotEmpty(t, rows)
	assert.Equal(t, map[uint64]int{millis(midnight): len(rows)}, seenTimes(rows), "points of 10:00 yesterday are stamped at the start of today")
}

func TestSetsAreWrittenOncePerWindow(t *testing.T) {
	cfg := testConfig()
	cfg.Cache.Window = time.Hour
	cfg.Cache.PreWriteWindow = 0
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()
	push := func(clock string) []row {
		now := testDay.Truncate(24 * time.Hour).Add(mustClock(clock))
		setNow(e, now)
		require.NoError(t, e.WriteLogs(ctx, logsAt(now, checkout, map[string]any{"http.method": "GET"})))
		return w.take()
	}
	assert.Len(t, push("10:05"), 3)
	assert.Empty(t, push("10:50"))
	rows := push("11:05")
	assert.Len(t, rows, 3, "a new window writes the set and the resource again")
	at11 := millis(testDay.Truncate(24 * time.Hour).Add(mustClock("11:05")))
	assert.Equal(t, map[uint64]int{at11: 3}, seenTimes(rows))
}

func mustClock(clock string) time.Duration {
	t, err := time.Parse("15:04", clock)
	if err != nil {
		panic(err)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
}

// In the last hour of the day, a key seen again is written for the next day,
// at a time set by its hash. At midnight it is already cached, so the new day
// does not start with a burst of all sets.
func TestKeysAreWrittenAheadInThePreWriteWindow(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}
	fresh := map[string]any{"http.method": "PUT"}

	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	assert.Len(t, w.take(), 3)

	setNow(e, midnight.Add(-time.Hour))
	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(-time.Hour), checkout, known)))
	assert.Empty(t, w.take(), "at the start of the pre-write window almost no key is due")

	setNow(e, lastMillisecond)
	ld := logsAt(lastMillisecond, checkout, known)
	addRecord(ld, lastMillisecond, fresh)
	require.NoError(t, e.WriteLogs(ctx, ld))
	rows := w.take()
	assert.Equal(t, map[uint64]int{millis(midnight): 3, millis(lastMillisecond): 1}, seenTimes(rows),
		"the known set and the resource are written ahead; the new set PUT is written for today only")

	after := midnight.Add(30 * time.Second)
	setNow(e, after)
	ld = logsAt(after, checkout, known)
	addRecord(ld, after, fresh)
	require.NoError(t, e.WriteLogs(ctx, ld))
	rows = w.take()
	assert.Equal(t, []string{"{http.method=PUT}"}, setList(rows), "only the set that was not written ahead is written at midnight")
}

func addRecord(ld plog.Logs, ts time.Time, attrs map[string]any) {
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	_ = lr.Attributes().FromRaw(attrs)
}

func TestKeysWrittenAheadAreCachedWhenTheWindowChangesDuringTheInsert(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}
	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	w.take()

	setNow(e, lastMillisecond)
	e.mu.Lock()
	b := e.newBatch()
	addLogs(b, logsAt(lastMillisecond, checkout, known))
	e.mu.Unlock()
	require.Len(t, b.pendingAhead, 2, "the set and the resource")

	setNow(e, midnight.Add(time.Second))
	e.mu.Lock()
	e.release(e.newBatch())
	e.commit(b)
	e.mu.Unlock()

	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(time.Minute), checkout, known)))
	assert.Empty(t, w.take(), "the keys written ahead are the keys of the new window")
}

func TestSharedCacheKnowsKeysWrittenAhead(t *testing.T) {
	shared := newMapCache()
	a, wa := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	b, wb := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}

	require.NoError(t, a.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	setNow(a, lastMillisecond)
	require.NoError(t, a.WriteLogs(ctx, logsAt(lastMillisecond, checkout, known)))
	assert.Len(t, wa.take(), 6, "today and ahead")

	setNow(b, midnight.Add(time.Minute))
	require.NoError(t, b.WriteLogs(ctx, logsAt(midnight.Add(time.Minute), checkout, known)))
	assert.Empty(t, wb.take(), "collector A wrote these keys ahead for today")
}

// The daily sample of a field over the limit grows with the time of the day
// in the first hour, so a new day does not start with every sample at once.
func TestSampleIsSpreadOverTheStartOfTheDay(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	e.class.Store(&classification{overClosedDays: map[fieldID]struct{}{fieldIDOf(contextAttribute, "user.id"): {}}})
	ctx := context.Background()
	n := 0
	samples := func(after time.Duration) int {
		now := midnight.Add(after)
		setNow(e, now)
		ld := plog.NewLogs()
		for i := 0; i < 10; i++ {
			n++
			if i == 0 {
				ld = logsAt(now, checkout, map[string]any{"user.id": fmt.Sprintf("u%d", n)})
				continue
			}
			addRecord(ld, now, map[string]any{"user.id": fmt.Sprintf("u%d", n)})
		}
		require.NoError(t, e.WriteLogs(ctx, ld))
		count := 0
		for _, r := range w.take() {
			if r.p.name == "user.id" {
				count++
			}
		}
		return count
	}
	assert.Equal(t, 0, samples(30*time.Second), "30 s into the day, the share of a budget of 4 is 0")
	assert.Equal(t, 2, samples(30*time.Minute), "after half of the hour, half of the budget")
	assert.Equal(t, 2, samples(time.Hour), "after the hour, the rest of the budget")
	assert.Equal(t, 0, samples(2*time.Hour), "the budget of the day is spent")
}

func TestTrackerFullKeepsWritingWithinBounds(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", map[string]any{"http.method": "GET"}})))
	w.take()
	e.state.budget.limit = e.state.budget.used

	ld := logsOf(checkout)
	for i := 0; i < 10; i++ {
		addRecord(ld, testDay, map[string]any{"user.id": fmt.Sprintf("u%d", i)})
	}
	e.mu.Lock()
	b := e.newBatch()
	addLogs(b, ld)
	e.mu.Unlock()
	assert.Len(t, setList(b.rows), 10, "values that the full tracker cannot count stay in the hash")

	e.mu.Lock()
	b = e.newBatch()
	addLogs(b, logsOf(map[string]any{"service.name": "cart"}, logRecord{"10:00", map[string]any{"http.method": "POST"}}))
	e.mu.Unlock()
	assert.Equal(t, 1, b.stats.resourcesUntracked)
	assert.Equal(t, []string{"http.method=POST*"}, sets(b.rows)[overflowAttrsHash], "a resource without a state writes into its overflow set")
}

func TestSpentFieldGivesItsMemoryBack(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e, _ := newTestExporter(t, cfg, pipeline.SignalLogs)
	ld := logsOf(checkout)
	for i := 0; i < 20; i++ {
		addRecord(ld, testDay, map[string]any{"user.id": fmt.Sprintf("u%d", i)})
	}
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	st := e.state.tracker.fields[fieldIDOf(contextAttribute, "user.id")]
	require.NotNil(t, st)
	assert.True(t, st.spent)
	assert.Zero(t, st.values.len())
	assert.Nil(t, st.values.slots)
}

func TestOriginalBodyAttributeIsNotAField(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	require.NoError(t, e.WriteLogs(context.Background(), logsOf(checkout,
		logRecord{"10:00", map[string]any{"__signoz_original_body__": "{}", "a": "1"}})))
	assert.Equal(t, []string{"{a=1}"}, setList(w.take()))
}

func TestSetHashIgnoresTheOrderOfPairs(t *testing.T) {
	a, b, c := stringPair(contextAttribute, "a", "1"), stringPair(contextAttribute, "b", "2"), stringPair(contextAttribute, "a", "2")
	assert.Equal(t, setHash([]pair{a, b}), setHash([]pair{b, a}))
	assert.NotEqual(t, setHash([]pair{a, b}), setHash([]pair{c, b}))
	assert.NotEqual(t, setHash([]pair{a}), setHash([]pair{a, a}), "a repeated pair changes the set")
	assert.Greater(t, setHash(nil), overflowAttrsHash)
}

func TestWindowConfigIsValidated(t *testing.T) {
	for _, tc := range []struct {
		window, preWrite time.Duration
		ok               bool
	}{
		{24 * time.Hour, time.Hour, true},
		{time.Hour, 5 * time.Minute, true},
		{time.Hour, 0, true},
		{7 * time.Hour, 0, false},
		{30 * time.Second, 0, false},
		{time.Hour, time.Hour, false},
		{time.Hour, -time.Second, false},
	} {
		cfg := DefaultConfig()
		cfg.Cache.Window, cfg.Cache.PreWriteWindow = tc.window, tc.preWrite
		assert.Equal(t, tc.ok, cfg.Validate() == nil, "window %s, pre-write %s", tc.window, tc.preWrite)
	}
}

func TestMetricsKeysAreWrittenAhead(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	ctx := context.Background()
	require.NoError(t, e.WriteMetrics(ctx, testMetrics()))
	first := len(w.take())

	md := testMetrics()
	setTimestamps(md, lastMillisecond)
	setNow(e, lastMillisecond)
	require.NoError(t, e.WriteMetrics(ctx, md))
	rows := w.take()
	assert.Len(t, rows, first)
	assert.Equal(t, map[uint64]int{millis(midnight): first}, seenTimes(rows))

	setNow(e, midnight.Add(time.Minute))
	setTimestamps(md, midnight.Add(time.Minute))
	require.NoError(t, e.WriteMetrics(ctx, md))
	assert.Empty(t, w.take())
}

func setTimestamps(md pmetric.Metrics, ts time.Time) {
	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		switch m.Type() {
		case pmetric.MetricTypeSum:
			for j := 0; j < m.Sum().DataPoints().Len(); j++ {
				m.Sum().DataPoints().At(j).SetTimestamp(pcommon.NewTimestampFromTime(ts))
			}
		case pmetric.MetricTypeHistogram:
			for j := 0; j < m.Histogram().DataPoints().Len(); j++ {
				m.Histogram().DataPoints().At(j).SetTimestamp(pcommon.NewTimestampFromTime(ts))
			}
		}
	}
}
