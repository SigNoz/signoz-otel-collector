package fieldvalues

import (
	"context"
	"fmt"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// overField is a field and a UTC day on which the field had more values than
// its limit. It is one row of field_days_over_limit.
type overField struct {
	ctx  fieldContext
	name string
	day  uint64
}

// classification is what the database knows about the fields of one signal:
// the fields over the limit and the fields that are in use.
type classification struct {
	// over holds the fields with a row in field_days_over_limit on one of the
	// days of the lookback, each with its last such day.
	over map[fieldID]overField
	// known are the fields over the limit, and then the other fields of
	// yesterday and today with the most rows first. They get their field
	// places first at the start of a day. It is read once per day.
	known    []fieldID
	knownDay uint64
	// endedDay is the last day at whose start the read for the day that
	// ended ran.
	endedDay uint64
}

func (c *classification) isOver(f fieldID) bool {
	if c == nil {
		return false
	}
	_, ok := c.over[f]
	return ok
}

// The day is read as a number: the driver gives a Date as a time in the time
// zone of the server.
const flaggedQuery = `SELECT field_context, field_name, toUInt16(max(day))
FROM signoz_metadata.distributed_field_days_over_limit
WHERE signal = ? AND source = ? AND day >= ?
GROUP BY field_context, field_name`

// rowsOfValues leaves out the rows that are not values of a field: the link
// rows of metrics, and the key rows of the resource fields of a metric.
const rowsOfValues = `signal = ? AND source = ? AND toDate(last_seen, 'UTC') >= ?
      AND field_name != '` + linkField + `' AND NOT (metric_name != '' AND field_context = 'resource')`

// The inner statement has the shape of the projection p_field_values, one
// row for each value, so that ClickHouse can read the projection in place of
// the rows of the sets. A condition on last_seen itself, or a subquery in
// place of the list of fields, makes ClickHouse read the rows of the sets.
const overLimitQuery = `SELECT field_context, field_name
FROM
(
    SELECT field_context, field_name, string_value, number_value
    FROM signoz_metadata.distributed_field_values_sets
    WHERE ` + rowsOfValues + `%s
    GROUP BY field_context, field_name, string_value, number_value
)
GROUP BY field_context, field_name
HAVING count() > if(field_context = 'resource', ?, ?)`

const notFlagged = `
      AND (field_context, field_name) NOT IN ?`

const knownFieldsQuery = `SELECT field_context, field_name
FROM signoz_metadata.distributed_field_values_sets
WHERE ` + rowsOfValues + `%s
GROUP BY field_context, field_name
ORDER BY count() DESC
LIMIT ?`

const overInsertQuery = "INSERT INTO signoz_metadata.distributed_field_days_over_limit (signal, source, field_context, field_name, day)"

type classifier struct {
	conn          driver.Conn
	signal        string
	source        string
	lookbackDays  int
	recordLimit   uint64
	resourceLimit uint64
	maxFields     int
	// endedWindow is the time after the start of a UTC day in which the read
	// for the day that ended is correct. Later, that read also counts the
	// values of the new day.
	endedWindow time.Duration
}

// refresh reads the fields over the limit and writes a row for each field
// that is new over the limit. own holds the fields that this collector saw
// over the limit by its own count. writeErr is the error of the write: the
// classification is valid without the rows, and the next refresh finds the
// fields again.
func (c *classifier) refresh(ctx context.Context, prev *classification, now time.Time, own []overField) (next *classification, writeErr, err error) {
	today := utcDay(now)
	over, err := c.flagged(ctx, today)
	if err != nil {
		return nil, nil, err
	}
	next = &classification{over: over}
	var rows []overField
	flag := func(fields []overField) {
		for _, f := range fields {
			id := fieldIDOf(f.ctx, f.name)
			if known, ok := over[id]; ok && known.day >= f.day {
				continue
			}
			over[id] = f
			rows = append(rows, f)
		}
	}
	flag(own)
	found, err := c.overLimit(ctx, today, over)
	if err != nil {
		return nil, nil, err
	}
	flag(found)
	if prev != nil {
		next.endedDay = prev.endedDay
	}
	if next.endedDay != today && now.Sub(dayTime(today)) <= c.endedWindow {
		found, err := c.overLimit(ctx, today-1, over)
		if err != nil {
			return nil, nil, err
		}
		flag(found)
		next.endedDay = today
	}
	if prev != nil && prev.knownDay == today {
		next.known, next.knownDay = prev.known, today
	} else {
		known, err := c.knownFields(ctx, today-1, over)
		if err != nil {
			return nil, nil, err
		}
		next.known, next.knownDay = known, today
	}
	return next, c.write(ctx, rows), nil
}

func (c *classifier) flagged(ctx context.Context, today uint64) (map[fieldID]overField, error) {
	rows, err := c.conn.Query(ctx, flaggedQuery, c.signal, c.source, dayString(today-uint64(c.lookbackDays-1)))
	if err != nil {
		return nil, fmt.Errorf("query fields with a day over the limit: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[fieldID]overField)
	for rows.Next() {
		var (
			ctxName, name string
			day           uint16
		)
		if err := rows.Scan(&ctxName, &name, &day); err != nil {
			return nil, err
		}
		fc, ok := parseFieldContext(ctxName)
		if !ok {
			return nil, fmt.Errorf("unknown field context %q", ctxName)
		}
		out[fieldIDOf(fc, name)] = overField{ctx: fc, name: name, day: uint64(day)}
	}
	return out, rows.Err()
}

// valueRows gives the arguments of rowsOfValues for the rows since the start
// of day, and the condition that leaves out the fields in skip. The fields
// over the limit hold most of the values, so the reads leave them out.
func (c *classifier) valueRows(day uint64, skip map[fieldID]overField) (clause string, args []any) {
	args = []any{c.signal, c.source, dayString(day)}
	if len(skip) == 0 {
		return "", args
	}
	fields := make([]clickhouse.GroupSet, 0, len(skip))
	for _, f := range skip {
		fields = append(fields, clickhouse.GroupSet{Value: []any{f.ctx.String(), f.name}})
	}
	return notFlagged, append(args, fields)
}

// overLimit returns the fields with more values than their limit since the
// start of day, without the fields in skip. The result has day as its day.
func (c *classifier) overLimit(ctx context.Context, day uint64, skip map[fieldID]overField) ([]overField, error) {
	clause, args := c.valueRows(day, skip)
	args = append(args, c.resourceLimit, c.recordLimit)
	rows, err := c.conn.Query(ctx, fmt.Sprintf(overLimitQuery, clause), args...)
	if err != nil {
		return nil, fmt.Errorf("query fields over the limit: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []overField
	for rows.Next() {
		fc, name, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, overField{ctx: fc, name: name, day: day})
	}
	return out, rows.Err()
}

func (c *classifier) knownFields(ctx context.Context, since uint64, over map[fieldID]overField) ([]fieldID, error) {
	clause, args := c.valueRows(since, over)
	rows, err := c.conn.Query(ctx, fmt.Sprintf(knownFieldsQuery, clause), append(args, c.maxFields)...)
	if err != nil {
		return nil, fmt.Errorf("query known fields: %w", err)
	}
	defer func() { _ = rows.Close() }()
	known := make([]fieldID, 0, len(over))
	for f := range over {
		known = append(known, f)
	}
	for rows.Next() {
		fc, name, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		known = append(known, fieldIDOf(fc, name))
	}
	return known, rows.Err()
}

func (c *classifier) write(ctx context.Context, fields []overField) error {
	if len(fields) == 0 {
		return nil
	}
	stmt, err := c.conn.PrepareBatch(ctx, overInsertQuery)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, f := range fields {
		if err := stmt.Append(c.signal, c.source, f.ctx.String(), f.name, dayString(f.day)); err != nil {
			return err
		}
	}
	return stmt.Send()
}

func scanField(rows driver.Rows) (fieldContext, string, error) {
	var ctxName, name string
	if err := rows.Scan(&ctxName, &name); err != nil {
		return 0, "", err
	}
	fc, ok := parseFieldContext(ctxName)
	if !ok {
		return 0, "", fmt.Errorf("unknown field context %q", ctxName)
	}
	return fc, name, nil
}

func dayTime(day uint64) time.Time {
	return time.Unix(int64(day)*86400, 0).UTC()
}

func dayString(day uint64) string {
	return dayTime(day).Format(time.DateOnly)
}
