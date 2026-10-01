package fieldvalues

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// classification is what the database knows about the fields of one signal:
// the fields over the limit and the fields that are in use.
type classification struct {
	// overClosedDays holds the fields over the limit on one of the closed days
	// of the lookback. It is read once per day.
	overClosedDays map[fieldKey]struct{}
	closedDay      uint64
	// overToday holds the fields over the limit today, read every refresh.
	overToday map[fieldKey]struct{}
	// known are yesterday's fields, ranked by holders. They get their field
	// places first at the start of a day.
	known []fieldKey
}

func (c *classification) isOver(fk fieldKey) bool {
	if c == nil {
		return false
	}
	if _, ok := c.overToday[fk]; ok {
		return true
	}
	_, ok := c.overClosedDays[fk]
	return ok
}

const overLimitQuery = `SELECT field_context, field_name
FROM
(
    SELECT field_context, field_name, day, uniqExact((string_value, number_value)) AS distinct_values
    FROM signoz_metadata.distributed_field_values_daily
    WHERE signal = ? AND source = ? AND metric_name = '' AND day >= toDate(now(), 'UTC') - ? AND day <= toDate(now(), 'UTC') - ?
    GROUP BY field_context, field_name, day
    HAVING distinct_values > if(field_context = 'resource', ?, ?)
)
GROUP BY field_context, field_name`

const knownFieldsQuery = `SELECT field_context, field_name
FROM signoz_metadata.distributed_field_values_daily
WHERE signal = ? AND source = ? AND metric_name = '' AND day = toDate(now(), 'UTC') - 1
GROUP BY field_context, field_name
ORDER BY uniqMerge(holders) DESC
LIMIT ?`

type classifier struct {
	conn          driver.Conn
	signal        string
	source        string
	lookbackDays  int
	recordLimit   uint64
	resourceLimit uint64
	maxFields     int
}

// refresh reads today's counts, and the closed days and the known fields when
// the day changed since prev.
func (c *classifier) refresh(ctx context.Context, prev *classification, today uint64) (*classification, error) {
	next := &classification{}
	if prev != nil && prev.closedDay == today {
		next.overClosedDays = prev.overClosedDays
		next.known = prev.known
		next.closedDay = today
	} else {
		over, err := c.overLimit(ctx, c.lookbackDays-1, 1)
		if err != nil {
			return nil, err
		}
		known, err := c.knownFields(ctx)
		if err != nil {
			return nil, err
		}
		next.overClosedDays, next.known, next.closedDay = over, known, today
	}
	over, err := c.overLimit(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	next.overToday = over
	return next, nil
}

func (c *classifier) overLimit(ctx context.Context, fromDaysAgo, toDaysAgo int) (map[fieldKey]struct{}, error) {
	rows, err := c.conn.Query(ctx, overLimitQuery, c.signal, c.source, fromDaysAgo, toDaysAgo, c.resourceLimit, c.recordLimit)
	if err != nil {
		return nil, fmt.Errorf("query fields over the limit: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[fieldKey]struct{})
	for rows.Next() {
		fk, err := scanFieldKey(rows)
		if err != nil {
			return nil, err
		}
		out[fk] = struct{}{}
	}
	return out, rows.Err()
}

func (c *classifier) knownFields(ctx context.Context) ([]fieldKey, error) {
	rows, err := c.conn.Query(ctx, knownFieldsQuery, c.signal, c.source, c.maxFields)
	if err != nil {
		return nil, fmt.Errorf("query known fields: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var known []fieldKey
	for rows.Next() {
		fk, err := scanFieldKey(rows)
		if err != nil {
			return nil, err
		}
		known = append(known, fk)
	}
	return known, rows.Err()
}

func scanFieldKey(rows driver.Rows) (fieldKey, error) {
	var ctxName, name string
	if err := rows.Scan(&ctxName, &name); err != nil {
		return fieldKey{}, err
	}
	fc, ok := parseFieldContext(ctxName)
	if !ok {
		return fieldKey{}, fmt.Errorf("unknown field context %q", ctxName)
	}
	return fieldKey{ctx: fc, name: name}, nil
}
