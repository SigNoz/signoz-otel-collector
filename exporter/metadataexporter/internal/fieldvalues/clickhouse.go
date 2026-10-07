package fieldvalues

import (
	"context"
	"time"

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
}

func (w *clickhouseWriter) write(ctx context.Context, rows []row) error {
	stmt, err := w.conn.PrepareBatch(ctx, insertQuery, driver.WithReleaseConnection())
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
