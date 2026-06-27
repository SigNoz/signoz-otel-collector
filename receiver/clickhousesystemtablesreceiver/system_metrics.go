package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ViewRefreshRow is one row of system.view_refreshes, per refreshable MV per replica.
type ViewRefreshRow struct {
	Hostname       string  `ch:"hostname"`
	Database       string  `ch:"database"`
	View           string  `ch:"view"`
	LastSuccessAge int64   `ch:"last_success_age"`
	LastDuration   float64 `ch:"last_duration"`
	Exception      uint8   `ch:"exception"`
	Retry          int64   `ch:"retry"`
	Progress       float64 `ch:"progress"`
}

// systemTableExpr wraps the table in clusterAllReplicas when a cluster is configured.
func systemTableExpr(clusterName, table string) string {
	if len(clusterName) > 0 {
		return fmt.Sprintf("clusterAllReplicas('%s', %s)", clusterName, table)
	}
	return table
}

func scrapeViewRefreshesTable(
	ctx context.Context, db driver.Conn, clusterName string,
) ([]ViewRefreshRow, error) {
	// last_success_time / last_success_duration_ms are Nullable: a view that has
	// not succeeded yet yields NULL. Coalesce to -1 so the row scans, and skip
	// recording the age datapoint for it (so a "never succeeded" view shows up as
	// missing data rather than a bogus age).
	query := fmt.Sprintf(`
		select
			hostName() as hostname,
			database,
			view,
			toInt64(ifNull(dateDiff('second', last_success_time, now()), -1)) as last_success_age,
			ifNull(last_success_duration_ms, 0) / 1000.0 as last_duration,
			toUInt8(exception != '') as exception,
			toInt64(retry) as retry,
			progress
		from %s
	`, systemTableExpr(clusterName, "system.view_refreshes"))

	return scanRows[ViewRefreshRow](ctx, db, query, "system.view_refreshes")
}

func scanRows[T any](ctx context.Context, db driver.Conn, query, label string) ([]T, error) {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("couldn't query %s: %w", label, err)
	}
	defer rows.Close()

	result := []T{}
	for rows.Next() {
		var row T
		if err := rows.ScanStruct(&row); err != nil {
			return nil, fmt.Errorf("couldn't scan %s row: %w", label, err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
