package fieldvalues

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvaluestest"
)

func TestLogKeysHaveTheStoredDataTypes(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ld := logsOf(map[string]any{"service.name": "checkout", "process.pid": int64(7)}, logRecord{"10:00", map[string]any{
		"http.method": "GET",
		"http.status": int64(200),
		"duration":    0.25,
		"retried":     true,
		"empty":       "",
		"ratio":       math.NaN(),
		"long":        strings.Repeat("x", 1000),
		"http":        map[string]any{"route": "/cart"},
	}})
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	assert.Equal(t, []string{
		"attribute duration float64",
		"attribute empty string",
		"attribute http string",
		"attribute http.method string",
		"attribute http.status int64",
		"attribute long string",
		"attribute ratio float64",
		"attribute retried bool",
		"resource process.pid string",
		"resource service.name string",
	}, w.takeKeys(e), "a key needs no value that can be stored; a map is one JSON string, and resource fields are strings")
}

// The traces exporter stores a map attribute of a span as one attribute for
// each leaf, and the attributes of events and resources as strings.
func TestSpanKeysHaveTheStoredNames(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalTraces)
	td := spansOf(map[string]any{"exception.code": int64(7), "detail": map[string]any{"a": "b"}})
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	require.NoError(t, span.Attributes().FromRaw(map[string]any{
		"http": map[string]any{"route": "/cart", "codes": []any{int64(200), "x"}, "tls": map[string]any{"ok": true}},
		"tags": []any{"a", "b"},
	}))
	require.NoError(t, e.WriteTraces(context.Background(), td))
	var keys []string
	for _, k := range w.takeKeys(e) {
		if !strings.HasPrefix(k, "span ") && !strings.HasPrefix(k, "scope ") {
			keys = append(keys, k)
		}
	}
	assert.Equal(t, []string{
		"attribute http.codes.0 int64",
		"attribute http.codes.1 string",
		"attribute http.route string",
		"attribute http.tls.ok bool",
		"attribute tags string",
		"event detail string",
		"event exception.code string",
		"event name string",
		"resource service.name string",
	}, keys)
}

func TestKeysOfASourceAreItsOwn(t *testing.T) {
	cfg := testConfig()
	cfg.Source = "audit"
	rows := &fakeWriter{}
	e, _ := newTestExporterWith(t, cfg, Settings{Signal: pipeline.SignalLogs}, rows)
	require.Equal(t, "audit", e.cfg.Source)
	require.NoError(t, e.WriteLogs(context.Background(), logsOf(checkout, logRecord{"10:00", map[string]any{"principal.id": "u1"}})))
	assert.Contains(t, rows.takeKeys(e), "attribute principal.id string")
}

// When the list of waiting keys is full, a key is not queued. The count shows
// it, and the key is queued at its next record with room.
func TestKeyThatIsNotQueuedIsCounted(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ks := e.state.keys
	for i := uint64(1); len(ks.pending) < maxPendingKeys; i++ {
		ks.pending[i] = fieldKey{}
	}
	p := stringPair(contextAttribute, "http.method", "GET")
	ks.see(&p, emptyNameHash, "")
	assert.Equal(t, 1, ks.notQueued)
	clear(ks.pending)
	ks.see(&p, emptyNameHash, "")
	assert.Equal(t, []string{"attribute http.method string"}, w.takeKeys(e))
}

func TestMetricKeysHaveTheMetricName(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	require.NoError(t, e.WriteMetrics(context.Background(), testMetrics()))
	assert.Equal(t, []string{
		"http_requests attribute code string",
		"http_requests attribute method string",
		"http_requests resource k8s.pod.name string",
		"http_requests resource service.name string",
		"http_requests scope library.lang string",
		"latency.count attribute route string",
		"latency.count resource k8s.pod.name string",
		"latency.count resource service.name string",
		"latency.count scope library.lang string",
		"resource k8s.pod.name string",
		"resource service.name string",
	}, w.takeKeys(e))
}

func TestKeyIsWrittenOncePerDayAfterAnInsertThatPassed(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	push := func(at time.Time) {
		setNow(e, at)
		require.NoError(t, e.WriteLogs(ctx, logsAt(at, map[string]any{"service.name": "checkout"}, map[string]any{"http.method": "GET"})))
	}
	keys := []string{"attribute http.method string", "resource service.name string"}

	w.failKeys = 1
	push(testDay)
	assert.Empty(t, w.takeKeys(e), "the insert failed")
	assert.Equal(t, keys, w.takeKeys(e), "the keys of a failed insert are inserted again, with no new record")

	push(testDay.Add(time.Hour))
	assert.Empty(t, w.takeKeys(e), "the keys have their row of today")

	push(midnight.Add(time.Second))
	assert.Equal(t, keys, w.takeKeys(e), "a new day writes each key again, also before the own time of its sets")
}

// The key list does not depend on the limits of the values, on the memory, or
// on a failed insert.
func TestKeysAreTheSameWithSmallAndLargeLimits(t *testing.T) {
	small := testConfig()
	small.Limits.MaxRecordFieldValues = 1
	small.Limits.MaxResourceFieldValues = 1
	small.Limits.MaxValueBytes = 1
	small.Limits.MaxFieldsPerSignal = 1
	small.Limits.MaxSetsPerResource = 1
	small.Limits.MaxOutsidePairsPerResource = 1
	large := testConfig()
	large.Cache.MaxBytes = 256 << 20

	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		t.Run(signal.String(), func(t *testing.T) {
			var want []string
			for i, cfg := range []Config{large, small} {
				e, w := newTestExporter(t, cfg, signal)
				if i == 1 {
					w.fail, w.failKeys = 2, 1
				}
				gen := fieldvaluestest.NewGenerator(9, testDay)
				for batch := 0; batch < 5; batch++ {
					switch signal {
					case pipeline.SignalLogs:
						require.NoError(t, e.WriteLogs(context.Background(), gen.Logs(500)))
					case pipeline.SignalTraces:
						require.NoError(t, e.WriteTraces(context.Background(), gen.Traces(500)))
					case pipeline.SignalMetrics:
						require.NoError(t, e.WriteMetrics(context.Background(), gen.Metrics(500)))
					}
					if i == 1 && batch == 0 {
						// No key cache room: every set and pair is left out.
						e.mu.Lock()
						e.state.cache.limit = [2]int{}
						e.mu.Unlock()
					}
				}
				keys := w.takeKeys(e)
				keys = append(keys, w.takeKeys(e)...)
				require.NotEmpty(t, keys)
				if i == 0 {
					want = keys
					continue
				}
				assert.ElementsMatch(t, want, keys)
			}
		})
	}
}
