package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Used by the receiver for working with clickhouse.
// Helps mock things for tests
type clickhouseQuerier interface {
	// Scrape query_log table for rows with minEventTs <= event_time < maxEventTs
	scrapeQueryLog(
		ctx context.Context, minEventTs uint32, maxEventTs uint32,
	) ([]QueryLog, error)

	// Get unix epoch time now at the server (seconds)
	unixTsNow(context.Context) (uint32, error)
}

type querrierImpl struct {
	db driver.Conn

	// Cluster name to use for scraping query log from clustered deployments.
	// If ClusterName is specified, the scrape will target `clusterAllReplicas(clusterName, system.query_log)`
	clusterName string
}

var _ clickhouseQuerier = (*querrierImpl)(nil)

func newClickhouseQuerrier(db driver.Conn, clusterName string) *querrierImpl {
	return &querrierImpl{
		db:          db,
		clusterName: clusterName,
	}
}

func (q *querrierImpl) scrapeQueryLog(
	ctx context.Context, minTs uint32, maxTs uint32,
) ([]QueryLog, error) {
	return scrapeQueryLogTable(ctx, q.db, q.clusterName, minTs, maxTs)
}

func (q *querrierImpl) unixTsNow(ctx context.Context) (
	uint32, error,
) {
	var serverTsNow uint32
	row := q.db.QueryRow(ctx, `select toUnixTimestamp(now())`)
	if err := row.Scan(&serverTsNow); err != nil {
		return 0, fmt.Errorf(
			"couldn't query current timestamp at clickhouse server: %w", err,
		)
	}
	return serverTsNow, nil
}
