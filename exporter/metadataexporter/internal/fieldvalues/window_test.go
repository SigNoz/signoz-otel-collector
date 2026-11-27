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

	"github.com/SigNoz/signoz-otel-collector/constants"
)

var (
	midnight        = testDay.Truncate(24 * time.Hour).Add(24 * time.Hour)
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
	cfg := testConfig()
	cfg.Cache.PreWriteWindow = 0
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
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

// A set of yesterday is written again in the first hour of the day, at a time
// set by its hash, so the new day does not start with a burst of all sets.
// Nothing is written for the new day before a record of that day.
func TestKnownSetsAreWrittenAgainSpreadOverTheStartOfTheDay(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}
	push := func(at time.Time, attrs map[string]any) []row {
		setNow(e, at)
		require.NoError(t, e.WriteLogs(ctx, logsAt(at, checkout, attrs)))
		return w.take()
	}

	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	assert.Len(t, w.take(), 3)
	assert.Empty(t, push(lastMillisecond, known), "a set seen at the end of the day gets no row of the next day")

	rows := push(midnight.Add(30*time.Second), map[string]any{"http.method": "PUT"})
	assert.Equal(t, []string{"{http.method=PUT}"}, setList(rows), "a new set is written at once")

	pushes := map[uint64]int{}
	for at := midnight; !at.After(midnight.Add(time.Hour)); at = at.Add(time.Minute) {
		for _, r := range push(at, known) {
			assert.Equal(t, millis(at), r.seenMillis)
			pushes[r.seenMillis]++
		}
	}
	counts := make([]int, 0, len(pushes))
	for at, n := range pushes {
		assert.Greater(t, at, millis(midnight), "no key is written at the start of the day")
		counts = append(counts, n)
	}
	assert.ElementsMatch(t, []int{1, 2}, counts, "the set and the resource are each written once, at their own times")
}

// A record that is the only one of its set today, and comes before the time
// of the set, gives no row today: the set has its row of yesterday. A read
// that starts today does not find this set.
func TestSetThatStopsBeforeItsOwnTimeHasNoRowToday(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}
	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	w.take()

	setNow(e, midnight.Add(time.Second))
	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(time.Second), checkout, known)))
	assert.Empty(t, w.take())
}

func addRecord(ld plog.Logs, ts time.Time, attrs map[string]any) {
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	_ = lr.Attributes().FromRaw(attrs)
}

func TestKeysAreCachedWhenTheWindowChangesDuringTheInsert(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}

	setNow(e, lastMillisecond)
	e.mu.Lock()
	b := e.newBatch()
	addLogs(b, logsAt(lastMillisecond, checkout, known))
	e.mu.Unlock()
	require.Len(t, b.pending, 2, "the set and the resource")

	setNow(e, midnight.Add(time.Second))
	e.mu.Lock()
	e.release(e.newBatch())
	e.commit(b)
	e.mu.Unlock()
	assert.Equal(t, [2]int{0, 0}, e.state.cache.used, "the keys do not count for the new window")

	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(time.Second), checkout, known)))
	assert.Empty(t, w.take(), "the keys are known as keys of the last window")
}

func TestSharedCacheKnowsTheKeysOfTheLastWindow(t *testing.T) {
	shared := newMapCache()
	a, wa := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	b, wb := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, Shared: shared}, &fakeWriter{})
	ctx := context.Background()
	known := map[string]any{"http.method": "GET"}

	require.NoError(t, a.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", known})))
	assert.Len(t, wa.take(), 3)

	setNow(b, midnight.Add(time.Second))
	require.NoError(t, b.WriteLogs(ctx, logsAt(midnight.Add(time.Second), checkout, known)))
	assert.Empty(t, wb.take(), "before the own time of a key, collector B reads the window in which collector A wrote it")

	setNow(b, midnight.Add(2*time.Hour))
	require.NoError(t, b.WriteLogs(ctx, logsAt(midnight.Add(2*time.Hour), checkout, known)))
	assert.Len(t, wb.take(), 3, "then the key is new in the window of today")
}

// The daily sample of a field over the limit grows with the time of the day
// in the first hour, so a new day does not start with every sample at once.
func TestSampleIsSpreadOverTheStartOfTheDay(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	e.class.Store(&classification{over: map[fieldID]overField{fieldIDOf(contextAttribute, "user.id"): {ctx: contextAttribute, name: "user.id"}}})
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
		logRecord{"10:00", map[string]any{constants.OriginalBodyAttributeKey: "{}", "a": "1"}})))
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

func TestMetricsKeysAreWrittenAgainInTheFirstHourOfTheDay(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	ctx := context.Background()
	require.NoError(t, e.WriteMetrics(ctx, testMetrics()))
	first := len(w.take())
	push := func(at time.Time) []row {
		md := testMetrics()
		setTimestamps(md, at)
		setNow(e, at)
		require.NoError(t, e.WriteMetrics(ctx, md))
		return w.take()
	}
	assert.Empty(t, push(lastMillisecond), "no row of the next day")

	total := 0
	for at := midnight; !at.After(midnight.Add(time.Hour)); at = at.Add(time.Minute) {
		for _, r := range push(at) {
			assert.Equal(t, millis(at), r.seenMillis)
			total++
		}
	}
	assert.Equal(t, first, total)
	assert.Empty(t, push(midnight.Add(2*time.Hour)))
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
