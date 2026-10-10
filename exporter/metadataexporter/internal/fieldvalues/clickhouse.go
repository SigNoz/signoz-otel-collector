package fieldvalues

import (
	"context"
	"sync"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const insertQuery = "INSERT INTO signoz_metadata.distributed_field_values_sets (signal, source, metric_name, field_context, field_name, field_data_type, string_value, number_value, resource_hash, attrs_hash, in_hash, first_seen, last_seen, inserted_at)"

type rowWriter interface {
	write(ctx context.Context, rows []row) error
}

type clickhouseWriter struct {
	conn   driver.Conn
	signal string
	source string
	// asyncTimeout is the busy timeout of async inserts, or 0 for sync
	// inserts.
	asyncTimeout time.Duration
	settingsOnce sync.Once
	settings     clickhouse.Settings
}

// insertSettings turns on async inserts that the writer does not wait for:
// the server joins the inserts of asyncTimeout into one part per table. The
// adaptive timeout of ClickHouse 24.3 and later would flush sooner under a
// low load, so it is turned off where it exists.
func (w *clickhouseWriter) insertSettings() clickhouse.Settings {
	w.settingsOnce.Do(func() {
		if w.asyncTimeout <= 0 {
			return
		}
		w.settings = clickhouse.Settings{
			"async_insert":                 1,
			"wait_for_async_insert":        0,
			"async_insert_busy_timeout_ms": w.asyncTimeout.Milliseconds(),
		}
		if v, err := w.conn.ServerVersion(); err == nil && (v.Version.Major > 24 || (v.Version.Major == 24 && v.Version.Minor >= 3)) {
			w.settings["async_insert_use_adaptive_busy_timeout"] = 0
		}
	})
	return w.settings
}

func (w *clickhouseWriter) write(ctx context.Context, rows []row) error {
	if settings := w.insertSettings(); settings != nil {
		ctx = clickhouse.Context(ctx, clickhouse.WithSettings(settings))
	}
	stmt, err := w.conn.PrepareBatch(ctx, insertQuery)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	now := time.Now().Unix()
	for _, r := range rows {
		var number *float64
		str := r.p.str
		if r.p.typ == typeNumber {
			v := r.p.num
			number = &v
			str = ""
		}
		if err := stmt.Append(
			w.signal,
			w.source,
			r.metricName,
			r.p.ctx.String(),
			r.p.name,
			r.p.typ.String(),
			str,
			number,
			r.resourceHash,
			r.attrsHash,
			r.inHash,
			int64(r.seenMillis/1000),
			int64(r.seenMillis/1000),
			now,
		); err != nil {
			return err
		}
	}
	return stmt.Send()
}
