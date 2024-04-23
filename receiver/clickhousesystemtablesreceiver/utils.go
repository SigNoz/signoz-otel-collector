package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func newClickhouseClient(dsn string) (clickhouse.Conn, error) {
	// use empty database to create database
	ctx := context.Background()
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// setting maxOpenIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	options := &clickhouse.Options{
		Addr:         []string{dsnURL.Host},
		MaxOpenConns: 2,
		MaxIdleConns: 1,
	}

	if dsnURL.Query().Get("username") != "" {
		auth := clickhouse.Auth{
			Username: dsnURL.Query().Get("username"),
			Password: dsnURL.Query().Get("password"),
		}
		options.Auth = auth
	}

	if dsnURL.Query().Get("dial_timeout") != "" {
		dialTimeout, err := time.ParseDuration(dsnURL.Query().Get("dial_timeout"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse dial_timeout from dsn: %w", err)
		}
		options.DialTimeout = dialTimeout
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
