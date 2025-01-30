package ch

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type LogCommentContextKeyType string

const LogCommentKey LogCommentContextKeyType = "logComment"

type ClickhouseQuerySettings struct {
}

type clickhouseConnWrapper struct {
	conn     clickhouse.Conn
	settings ClickhouseQuerySettings
}

func NewClickhouseConnWrapper(conn clickhouse.Conn, settings ClickhouseQuerySettings) clickhouse.Conn {
	return &clickhouseConnWrapper{
		conn:     conn,
		settings: settings,
	}
}

func (c clickhouseConnWrapper) Close() error {
	return c.conn.Close()
}

func (c clickhouseConnWrapper) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c clickhouseConnWrapper) Stats() driver.Stats {
	return c.conn.Stats()
}

func (c clickhouseConnWrapper) addClickHouseSettings(ctx context.Context, query string) context.Context {
	settings := clickhouse.Settings{}

	logComment := c.getLogComment(ctx)
	if logComment != "" {
		settings["log_comment"] = logComment
	}

	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(settings))
	return ctx
}

func (c clickhouseConnWrapper) getLogComment(ctx context.Context) string {
	// Get the key-value pairs from context for log comment
	kv := ctx.Value(LogCommentKey)
	if kv == nil {
		return ""
	}

	logCommentKVs, ok := kv.(map[string]interface{})
	if !ok {
		return ""
	}

	logComment, _ := json.Marshal(logCommentKVs)

	return string(logComment)
}

func (c clickhouseConnWrapper) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return c.conn.Query(c.addClickHouseSettings(ctx, query), query, args...)
}

func (c clickhouseConnWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return c.conn.QueryRow(c.addClickHouseSettings(ctx, query), query, args...)
}

func (c clickhouseConnWrapper) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return c.conn.Select(c.addClickHouseSettings(ctx, query), dest, query, args...)
}

func (c clickhouseConnWrapper) Exec(ctx context.Context, query string, args ...interface{}) error {
	return c.conn.Exec(c.addClickHouseSettings(ctx, query), query, args...)
}

func (c clickhouseConnWrapper) AsyncInsert(ctx context.Context, query string, wait bool, args ...interface{}) error {
	return c.conn.AsyncInsert(c.addClickHouseSettings(ctx, query), query, wait, args...)
}

func (c clickhouseConnWrapper) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return c.conn.PrepareBatch(c.addClickHouseSettings(ctx, query), query, opts...)
}

func (c clickhouseConnWrapper) ServerVersion() (*driver.ServerVersion, error) {
	return c.conn.ServerVersion()
}

func (c clickhouseConnWrapper) Contributors() []string {
	return c.conn.Contributors()
}
