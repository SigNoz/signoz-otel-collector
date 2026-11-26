package fieldvalues

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

func onlyResourceState(t *testing.T, e *Writer) *resourceState {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.Len(t, e.state.resources.states, 1)
	for _, rs := range e.state.resources.states {
		return rs
	}
	return nil
}

// The logs and traces exporters store resource and scope attributes as
// strings, so the key tables have them as strings. A reader asks for the
// values of a field with the data type of its key.
func TestResourceAndScopeAttributesAreStrings(t *testing.T) {
	typed := func(rows []row) []string {
		var out []string
		for _, r := range rows {
			out = append(out, fmt.Sprintf("%s %s %s", r.p.ctx, r.p.typ, describe(r.p)))
		}
		sort.Strings(out)
		return out
	}

	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutInt("process.pid", 4242)
	rl.Resource().Attributes().PutBool("cloud.spot", true)
	rl.Resource().Attributes().PutDouble("host.load", 0.5)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutInt("scope.level", 3)
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(at("10:00"))
	lr.Attributes().PutInt("http.status_code", 200)
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	assert.Equal(t, []string{
		"attribute number http.status_code=200",
		"resource string cloud.spot=true",
		"resource string host.load=0.5",
		"resource string process.pid=4242",
		"scope string scope.level=3",
	}, typed(w.take()))

	e, w = newTestExporter(t, testConfig(), pipeline.SignalTraces)
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutInt("process.pid", 4242)
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().Attributes().PutBool("scope.debug", false)
	span := ss.Spans().AppendEmpty()
	span.SetStartTimestamp(at("10:00"))
	require.NoError(t, e.WriteTraces(context.Background(), td))
	var got []string
	for _, r := range typed(w.take()) {
		if r[:4] != "span" {
			got = append(got, r)
		}
	}
	assert.Equal(t, []string{"resource string process.pid=4242", "scope string scope.debug=false"}, got)
}

// Span event fields are always outside the hash, so a resource with many
// event values fills its limit of such pairs while it has few sets and no
// field to drop. Its sets stay exact, and only the pairs past the limit go
// to the overflow set.
func TestPairLimitAloneKeepsTheSetsExact(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxSetsPerResource = 1000
	cfg.Limits.MaxOutsidePairsPerResource = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalTraces)
	span := func(method string, exceptions ...string) ptrace.Traces {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "checkout")
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetStartTimestamp(at("10:00"))
		s.Attributes().PutStr("http.method", method)
		for _, v := range exceptions {
			s.Events().AppendEmpty().Attributes().PutStr("exception.type", v)
		}
		return td
	}
	methods := func(rows []row) []string {
		var out []string
		for _, r := range rows {
			if r.p.name == "http.method" {
				out = append(out, fmt.Sprintf("%s in_hash=%v overflow=%v", r.p.str, r.inHash, r.attrsHash == overflowAttrsHash))
			}
		}
		return out
	}
	ctx := context.Background()
	require.NoError(t, e.WriteTraces(ctx, span("GET", "E1", "E2", "E3", "E4")))
	w.take()
	rs := onlyResourceState(t, e)
	require.Equal(t, 3, rs.stepPairs, "the pair limit is full")

	require.NoError(t, e.WriteTraces(ctx, span("POST", "E5")))
	rows := w.take()
	assert.Equal(t, []string{"POST in_hash=true overflow=false"}, methods(rows), "a new set is exact")
	assert.False(t, rs.overflow)
	var overflowed []string
	for _, r := range rows {
		if r.attrsHash == overflowAttrsHash {
			overflowed = append(overflowed, describe(r.p))
		}
	}
	assert.Equal(t, []string{"exception.type=E5"}, overflowed, "only the new pair outside the hash goes to the overflow set")
}

// A failed insert writes no row, so it must not use up the limits of a
// resource: with ClickHouse down, the same sets come again in each push.
func TestFailedInsertsDoNotUseTheLimitsOfAResource(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxSetsPerResource = 4
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()
	twoSets := func() plog.Logs {
		return logsOf(checkout,
			logRecord{"10:00", map[string]any{"http.method": "GET", "status": "200"}},
			logRecord{"10:00", map[string]any{"http.method": "POST", "status": "200"}},
		)
	}
	w.fail = 30
	for i := 0; i < 30; i++ {
		require.NoError(t, e.WriteLogs(ctx, twoSets()))
	}
	rs := onlyResourceState(t, e)
	assert.Equal(t, 0, rs.stepSets)
	assert.Empty(t, rs.dropped)
	assert.False(t, rs.overflow)

	require.NoError(t, e.WriteLogs(ctx, twoSets()))
	assert.Equal(t, []string{"{http.method=GET, status=200}", "{http.method=POST, status=200}"}, setList(w.take()))
	assert.Equal(t, 2, rs.stepSets)
}

// A coarse step drops the field with the most values. It never drops a field
// of always_include.
func TestCoarseStepKeepsAlwaysIncludeFields(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxSetsPerResource = 4
	cfg.Limits.MaxOutsidePairsPerResource = 100
	cfg.AlwaysInclude = []string{"tenant.id"}
	e, w := newTestExporter(t, cfg, pipeline.SignalLogs)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", map[string]any{
			"tenant.id": fmt.Sprintf("t%d", i%3), "path": fmt.Sprintf("/p%d", i%2),
		}})))
	}
	inHash := map[string]map[bool]int{"tenant.id": {}, "path": {}}
	for _, r := range w.take() {
		if counts, ok := inHash[r.p.name]; ok {
			counts[r.inHash]++
		}
	}
	rs := onlyResourceState(t, e)
	assert.False(t, rs.isDropped(fieldIDOf(contextAttribute, "tenant.id")))
	assert.True(t, rs.isDropped(fieldIDOf(contextAttribute, "path")), "path has fewer values, and it is the only field that a step can drop")
	assert.Zero(t, inHash["tenant.id"][false], "tenant.id stays in the hash")
	assert.False(t, rs.overflow)
}
