package fieldvalues

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pipeline"
)

var testBodyLimits = BodyJSONLimits{MaxDepthTraverse: 22, MaxArrayElementsAllowed: 100, MaxKeysAtLevel: 1024}

func bodyOf(t *testing.T, raw map[string]any) pcommon.Value {
	t.Helper()
	v := pcommon.NewValueMap()
	require.NoError(t, v.Map().FromRaw(raw))
	return v
}

func TestBodyValuesFollowTheJSONWriterRules(t *testing.T) {
	body := bodyOf(t, map[string]any{
		"user":                                 map[string]any{"id": "u1", "plan": "pro", "beta": true},
		"items":                                []any{map[string]any{"sku": "a1"}, map[string]any{"sku": "b2"}},
		"tags":                                 []any{"x", 2},
		"nested":                               []any{[]any{"no"}},
		"message":                              "the message field is skipped",
		"0f8fad5b-d9cb-469f-a165-70867728950e": "a key that looks like an id is skipped",
	})
	assert.ElementsMatch(t, []string{
		"user.id=u1", "user.plan=pro", "user.beta=bool",
		"items[].sku=a1", "items[].sku=b2",
		"tags=x", "tags=2",
	}, BodyValues(body, testBodyLimits))
}

func TestBodyValuesLimits(t *testing.T) {
	deep := map[string]any{"leaf": "v"}
	for i := 0; i < 15; i++ {
		deep = map[string]any{"d": deep}
	}
	assert.Empty(t, BodyValues(bodyOf(t, deep), testBodyLimits), "the walk stops at the depth limit")

	wide := map[string]any{}
	for i := 0; i < 5; i++ {
		wide[strings.Repeat("k", i+1)] = "v"
	}
	assert.Empty(t, BodyValues(bodyOf(t, map[string]any{"m": wide}), BodyJSONLimits{MaxDepthTraverse: 22, MaxArrayElementsAllowed: 100, MaxKeysAtLevel: 4}))

	assert.Empty(t, BodyValues(bodyOf(t, map[string]any{"a": []any{1, 2, 3}}), BodyJSONLimits{MaxDepthTraverse: 22, MaxArrayElementsAllowed: 2, MaxKeysAtLevel: 1024}))

	assert.Empty(t, BodyValues(pcommon.NewValueStr("plain text body"), testBodyLimits), "only map bodies have paths")
}

func TestLogsBodyPairsJoinTheSet(t *testing.T) {
	e, w := newTestExporterWith(t, testConfig(), Settings{Signal: pipeline.SignalLogs, BodyJSON: &testBodyLimits}, &fakeWriter{})
	ld := logsOf(checkout, logRecord{"10:00", map[string]any{"http.method": "GET"}})
	require.NoError(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetEmptyMap().FromRaw(map[string]any{"user": map[string]any{"plan": "pro"}}))
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	rows := w.take()
	assert.Equal(t, []string{"{http.method=GET, user.plan=pro}"}, setList(rows))
	for _, r := range rows {
		if r.p.name == "user.plan" {
			assert.Equal(t, contextBody, r.p.ctx)
		}
	}

	e2, w2 := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	require.NoError(t, e2.WriteLogs(context.Background(), ld))
	assert.Equal(t, []string{"{http.method=GET}"}, setList(w2.take()), "without the JSON config there are no body pairs")
}
