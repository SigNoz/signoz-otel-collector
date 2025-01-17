package clickhousesystemtablesreceiver

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func newClickhouseClient(ctx context.Context, dsn string) (clickhouse.Conn, error) {
	options, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}

	return db, nil
}
