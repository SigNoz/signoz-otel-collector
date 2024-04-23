// import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"
package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Used by the receiver for working with clickhouse.
// Helps mock things for tests
type clickhouseQuerrier interface {
	scrapeQueryLog(
		ctx context.Context, minTs uint32, maxTs uint32,
	) ([]QueryLog, error)

	unixTsNow(context.Context) (uint32, error)
}

type querrierImpl struct {
	db driver.Conn
}

var _ clickhouseQuerrier = (*querrierImpl)(nil)

func newClickhouseQuerrier(db driver.Conn) *querrierImpl {
	return &querrierImpl{
		db: db,
	}
}

func (q *querrierImpl) scrapeQueryLog(
	ctx context.Context, minTs uint32, maxTs uint32,
) ([]QueryLog, error) {
	return scrapeQueryLogTable(ctx, q.db, minTs, maxTs)
}

// func (q *querrierImpl) scrapeQueryLogRecords(
// 	ctx context.Context, minTs uint32, maxTs uint32,
// ) (plog.LogRecordSlice, error) {
// 	res := plog.NewLogRecordSlice()

// 	queryLogs, err := scrapeQueryLogs(
// 		ctx, q.db, minTs, maxTs,
// 	)
// 	if err != nil {
// 		return res, err
// 	}

// 	for _, ql := range queryLogs {
// 		lr, err := ql.toLogRecord()
// 		if err != nil {
// 			return res, fmt.Errorf("couldn't convert to query_log to plog record: %w", err)
// 		}
// 		lr.CopyTo(res.AppendEmpty())
// 	}

// 	return res, nil
// }

func (q *querrierImpl) unixTsNow(ctx context.Context) (
	uint32, error,
) {
	var serverTsNow uint32
	row := q.db.QueryRow(ctx, `select toUnixTimestamp(now())`)
	if err := row.Scan(&serverTsNow); err != nil {
		return 0, fmt.Errorf("couldn't query current timestamp at clickhouse server: %w", err)
	}
	return serverTsNow, nil
}
