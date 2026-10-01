package fieldvalues

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

var testDay = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

type fakeWriter struct {
	rows []row
	fail int
}

func (w *fakeWriter) write(_ context.Context, rows []row) error {
	if w.fail > 0 {
		w.fail--
		return errors.New("insert failed")
	}
	w.rows = append(w.rows, rows...)
	return nil
}

func (w *fakeWriter) take() []row {
	rows := w.rows
	w.rows = nil
	return rows
}

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Cache.MaxBytes = 1 << 20
	return cfg
}

func newTestExporter(t *testing.T, cfg Config, signal pipeline.Signal) (*Writer, *fakeWriter) {
	t.Helper()
	return newTestExporterWith(t, cfg, Settings{Signal: signal}, &fakeWriter{})
}

func newTestExporterWith(t *testing.T, cfg Config, set Settings, w *fakeWriter) (*Writer, *fakeWriter) {
	t.Helper()
	set.Logger = zap.NewNop()
	tel, err := newTelemetry(componenttest.NewNopTelemetrySettings(), set.Signal.String(), cfg.Source)
	require.NoError(t, err)
	e := newWriter(cfg, set, w, tel)
	now := testDay
	e.now = func() time.Time { return now }
	return e, w
}

func setNow(e *Writer, now time.Time) {
	e.now = func() time.Time { return now }
}

// at gives a timestamp on testDay.
func at(clock string) pcommon.Timestamp {
	t, err := time.Parse("15:04", clock)
	if err != nil {
		panic(err)
	}
	day := testDay
	return pcommon.NewTimestampFromTime(time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC))
}

type logRecord struct {
	clock string
	attrs map[string]any
}

func logsOf(resource map[string]any, records ...logRecord) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	require.NoError(nil, rl.Resource().Attributes().FromRaw(resource))
	sl := rl.ScopeLogs().AppendEmpty()
	for _, r := range records {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(at(r.clock))
		_ = lr.Attributes().FromRaw(r.attrs)
	}
	return ld
}

func describe(p pair) string {
	if p.typ == typeNumber {
		return fmt.Sprintf("%s=%g", p.name, p.num)
	}
	return fmt.Sprintf("%s=%s", p.name, p.str)
}

// sets groups the rows of record sets by attrs_hash. Pairs in the hash are
// shown as "name=value", pairs outside the hash with a trailing "*".
func sets(rows []row) map[uint64][]string {
	out := make(map[uint64][]string)
	for _, r := range rows {
		if r.attrsHash == resourceAttrsHash {
			continue
		}
		s := describe(r.p)
		if !r.inHash {
			s += "*"
		}
		out[r.attrsHash] = append(out[r.attrsHash], s)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

func setList(rows []row) []string {
	var out []string
	for _, pairs := range sets(rows) {
		out = append(out, "{"+strings.Join(pairs, ", ")+"}")
	}
	sort.Strings(out)
	return out
}

func resourceRows(rows []row) []string {
	var out []string
	for _, r := range rows {
		if r.attrsHash != resourceAttrsHash {
			continue
		}
		s := describe(r.p)
		if !r.inHash {
			s += "*"
		}
		if r.metricName != "" {
			s = r.metricName + ":" + s
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
