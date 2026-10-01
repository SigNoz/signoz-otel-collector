package fieldvalues

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"
)

var checkout = map[string]any{"service.name": "checkout", "deployment.environment.name": "prod"}

func TestLogsWriteEachSetOncePerDay(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()

	ld := logsOf(checkout,
		logRecord{"10:05", map[string]any{"http.method": "GET"}},
		logRecord{"10:06", map[string]any{"http.method": "GET"}},
		logRecord{"10:07", map[string]any{"http.method": "POST"}},
	)
	require.NoError(t, e.WriteLogs(ctx, ld))
	rows := w.take()
	assert.Equal(t, []string{"{http.method=GET}", "{http.method=POST}"}, setList(rows))
	assert.Equal(t, []string{"deployment.environment.name=prod", "service.name=checkout"}, resourceRows(rows))

	require.NoError(t, e.WriteLogs(ctx, ld))
	assert.Empty(t, w.take(), "known sets and resources write nothing again on the same day")

	setNow(e, testDay.Add(24*time.Hour))
	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:05", map[string]any{"http.method": "GET"}})))
	rows = w.take()
	assert.Equal(t, []string{"{http.method=GET}"}, setList(rows), "a new day writes the set again")
	assert.Len(t, resourceRows(rows), 2)
}

func TestLogsCacheKeysOnlyAfterInsertSucceeds(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	ld := logsOf(checkout, logRecord{"10:05", map[string]any{"http.method": "GET"}})

	w.fail = 1
	require.NoError(t, e.WriteLogs(ctx, ld))
	assert.Empty(t, w.take())

	require.NoError(t, e.WriteLogs(ctx, ld))
	assert.Equal(t, []string{"{http.method=GET}"}, setList(w.take()))
}

// The example "a high-cardinality field" of the proposal: limit 3, budget 4.
func TestLogsHighCardinalityFieldKeepsDailySample(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()

	day1 := logsOf(checkout,
		logRecord{"08:00", map[string]any{"http.method": "GET", "user.id": "u1"}},
		logRecord{"08:01", map[string]any{"http.method": "POST", "user.id": "u2"}},
		logRecord{"08:02", map[string]any{"http.method": "GET", "user.id": "u3"}},
		logRecord{"08:03", map[string]any{"http.method": "GET", "user.id": "u4"}},
		logRecord{"08:04", map[string]any{"http.method": "GET", "user.id": "u5"}},
	)
	require.NoError(t, e.WriteLogs(ctx, day1))
	assert.Equal(t, []string{
		"{http.method=GET, user.id=u1}",
		"{http.method=GET, user.id=u3}",
		"{http.method=GET, user.id=u4*}",
		"{http.method=POST, user.id=u2}",
	}, setList(w.take()), "u4 passes the limit and goes into {GET} outside the hash; u5 is past the budget")

	// Day 1 showed 4 values in field_values_daily, so user.id starts day 2
	// outside the hash.
	e.class.Store(&classification{overClosedDays: map[fieldKey]struct{}{{ctx: contextAttribute, name: "user.id"}: {}}})
	setNow(e, testDay.Add(24*time.Hour))
	day2 := logsOf(checkout,
		logRecord{"08:00", map[string]any{"http.method": "GET", "user.id": "u6"}},
		logRecord{"08:01", map[string]any{"http.method": "POST", "user.id": "u7"}},
		logRecord{"08:02", map[string]any{"http.method": "GET", "user.id": "u8"}},
		logRecord{"08:03", map[string]any{"http.method": "GET", "user.id": "u9"}},
		logRecord{"08:04", map[string]any{"http.method": "GET", "user.id": "u10"}},
	)
	require.NoError(t, e.WriteLogs(ctx, day2))
	assert.Equal(t, []string{
		"{http.method=GET, user.id=u6*, user.id=u8*, user.id=u9*}",
		"{http.method=POST, user.id=u7*}",
	}, setList(w.take()))
}

// The example "a resource past its set limit" of the proposal: 2 sets per step.
func TestLogsCoarseSets(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxSetsPerResource = 2
	cfg.Limits.MaxOutsidePairsPerResource = 100
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	alb := map[string]any{"cloud.resource_id": "app/alb"}
	ctx := context.Background()

	push := func(clock string, attrs map[string]any) {
		require.NoError(t, e.WriteLogs(ctx, logsOf(alb, logRecord{clock, attrs})))
	}
	push("10:00", map[string]any{"http.method": "GET", "url.path": "/a"})
	push("10:01", map[string]any{"http.method": "GET", "url.path": "/b"})
	push("10:02", map[string]any{"http.method": "GET", "url.path": "/c"})
	push("11:00", map[string]any{"http.method": "GET", "url.path": "/d", "feature.flag": "on"})
	push("11:10", map[string]any{"http.method": "POST", "url.path": "/e", "feature.flag": "on"})
	push("11:20", map[string]any{"http.method": "GET", "url.path": "/f", "feature.flag": "off"})
	push("11:30", map[string]any{"http.method": "GET", "url.path": "/g", "feature.flag": "on"})

	assert.Equal(t, []string{
		"{feature.flag=off, http.method=GET*, url.path=/f*}",
		"{feature.flag=on, http.method=GET*, http.method=POST*, url.path=/e*, url.path=/g*}",
		"{feature.flag=on, http.method=GET, url.path=/d*}",
		"{http.method=GET, url.path=/a}",
		"{http.method=GET, url.path=/b}",
		"{http.method=GET, url.path=/c*}",
	}, setList(w.take()))
}

func TestLogsOverflowSetWhenCacheIsFull(t *testing.T) {
	cfg := testConfig()
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	// Room for 3 exact keys (one resource and two sets) and 3 reserve keys.
	e.state.cache.limit = [2]int{3, 3}
	ctx := context.Background()

	ld := logsOf(checkout,
		logRecord{"10:00", map[string]any{"http.method": "GET", "status": "200"}},
		logRecord{"10:01", map[string]any{"http.method": "POST", "status": "200"}},
		logRecord{"10:02", map[string]any{"http.method": "GET", "status": "500"}},
		logRecord{"10:03", map[string]any{"http.method": "POST", "status": "404"}},
	)
	e.mu.Lock()
	first := e.newBatch()
	first.addLogs(ld)
	e.mu.Unlock()
	assert.Equal(t, 1, first.stats.leftOut[reasonCacheFull], "the last record has two new pairs and the reserve has room for one")
	assert.Equal(t, 1, first.stats.resourcesOverflowed)

	require.NoError(t, e.WriteLogs(ctx, ld))
	rows := w.take()
	overflow := sets(rows)[overflowAttrsHash]
	require.Len(t, overflow, 3, "the reserve holds 3 pairs")
	assert.Subset(t, overflow, []string{"http.method=GET*", "status=500*"}, "the third set goes into the overflow set")
	lastRecord := 0
	for _, p := range overflow {
		if p == "http.method=POST*" || p == "status=404*" {
			lastRecord++
		}
	}
	assert.Equal(t, 1, lastRecord, "one pair of the last record fits; which one depends on the order of its attributes")
	assert.Contains(t, setList(rows), "{http.method=GET, status=200}")
	assert.Contains(t, setList(rows), "{http.method=POST, status=200}")
	e.mu.Lock()
	b := e.newBatch()
	b.addLogs(logsOf(checkout, logRecord{"10:04", map[string]any{"http.method": "PUT", "status": "200"}}))
	e.mu.Unlock()
	assert.Equal(t, 2, b.stats.leftOut[reasonCacheFull], "with the reserve full, the new pairs http.method=PUT and status=200 of the overflow set are left out")
}

func TestLogsResourceFieldLeavesIdentityButKeepsValues(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxResourceFieldValues = 2
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()
	event := func(version string) map[string]any {
		return map[string]any{"k8s.namespace.name": "shop", "k8s.object.kind": "Pod", "k8s.object.resource_version": version}
	}
	for _, v := range []string{"101", "102", "103", "104"} {
		require.NoError(t, e.WriteLogs(ctx, logsOf(event(v), logRecord{"10:00", map[string]any{"k8s.event.reason": "BackOff"}})))
	}
	rows := w.take()
	assert.Equal(t, []string{
		"k8s.namespace.name=shop", "k8s.namespace.name=shop", "k8s.namespace.name=shop",
		"k8s.object.kind=Pod", "k8s.object.kind=Pod", "k8s.object.kind=Pod",
		"k8s.object.resource_version=101", "k8s.object.resource_version=102",
		"k8s.object.resource_version=103*", "k8s.object.resource_version=104*",
	}, resourceRows(rows), "101 and 102 are resources of their own; from 103 on, the version leaves the identity and is a row of the merged resource")

	merged := map[uint64]bool{}
	for _, r := range rows {
		if r.p.name == "k8s.object.resource_version" && !r.inHash {
			merged[r.resourceHash] = true
		}
	}
	assert.Len(t, merged, 1, "103 and 104 belong to one merged resource")
}

func TestLogsValueRules(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxValueBytes = 8
	cfg.Limits.MaxFieldsPerSignal = 4
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()

	ld := logsOf(map[string]any{"service.name": "api"},
		logRecord{"10:00", map[string]any{"short": "ok", "long": "longer than eight", "empty": "", "count": 3, "ok": true, "extra": "x"}},
	)
	b := func() batchStats {
		e.mu.Lock()
		defer e.mu.Unlock()
		b := e.newBatch()
		b.addLogs(ld)
		return b.stats
	}()
	assert.Equal(t, 1, b.leftOut[reasonValueLength])
	assert.Equal(t, 1, b.leftOut[reasonFieldPlaces], "service.name and three of the four record fields take the 4 places; the last field has none")

	e2, w2 := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	require.NoError(t, e2.WriteLogs(ctx, logsOf(map[string]any{"service.name": "api"},
		logRecord{"10:00", map[string]any{"count": 3, "ok": true, "empty": ""}})))
	rows := w2.take()
	assert.Equal(t, []string{"{count=3, ok=true}"}, setList(rows))
	for _, r := range rows {
		if r.p.name == "count" {
			assert.Equal(t, typeNumber, r.p.typ)
		}
		if r.p.name == "ok" {
			assert.Equal(t, typeBool, r.p.typ)
		}
	}
	_ = w
}

func TestLogsTimestampsAreClamped(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ld := logsOf(checkout, logRecord{"10:00", map[string]any{"a": "1"}})
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	lr.SetTimestamp(0)
	lr.SetObservedTimestamp(at("11:00"))
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	for _, r := range w.take() {
		assert.Equal(t, uint64(at("11:00").AsTime().UnixMilli()), r.seenMillis, "a record time of 0 uses the observed time")
	}

	e2, w2 := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	future := logsOf(checkout, logRecord{"10:00", map[string]any{"a": "1"}})
	future.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetTimestamp(at("10:00") + 3600e9*1000)
	require.NoError(t, e2.WriteLogs(context.Background(), future))
	for _, r := range w2.take() {
		assert.Equal(t, uint64(testDay.UnixMilli()), r.seenMillis, "a time far in the future becomes the batch time")
	}
}

func TestLogsPairsOutsideTheHashAreBoundedPerResource(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxSetsPerResource = 2
	cfg.Limits.MaxOutsidePairsPerResource = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	alb := map[string]any{"cloud.resource_id": "app/alb"}
	ctx := context.Background()
	push := func(attrs map[string]any) {
		require.NoError(t, e.WriteLogs(ctx, logsOf(alb, logRecord{"10:00", attrs})))
	}
	push(map[string]any{"http.method": "GET", "url.path": "/a"})
	push(map[string]any{"http.method": "GET", "url.path": "/b"})
	// url.path leaves the hash; the set {GET} is known from now on, and its new
	// paths are pairs outside the hash.
	for _, path := range []string{"/c", "/d", "/e", "/f", "/g"} {
		push(map[string]any{"http.method": "GET", "url.path": path})
	}
	got := sets(w.take())
	var coarse, overflow []string
	for ah, pairs := range got {
		if ah == overflowAttrsHash {
			overflow = pairs
		} else if len(pairs) > 2 {
			coarse = pairs
		}
	}
	assert.Equal(t, []string{"http.method=GET", "url.path=/c*", "url.path=/d*", "url.path=/e*"}, coarse, "3 pairs outside the hash fit in the step")
	assert.Equal(t, []string{"url.path=/f*", "url.path=/g*"}, overflow, "the rest go into the overflow set")
}
