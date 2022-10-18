package usage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Options provides options for LogExporter
type Options struct {
	// ReportingInterval is a time interval between two successive metrics
	// export.
	ReportingInterval time.Duration
}

type Usage struct {
	ID       string    `ch:"id"`
	Datetime time.Time `ch:"datetime"`
	Count    int64     `ch:"count"`
	Size     int64     `ch:"size"`
}

func Init(db clickhouse.Conn, databaseName string) error {
	query := fmt.Sprintf(
		`
		CREATE TABLE IF NOT EXISTS %s.usage (
			id String,
			datetime DateTime CODEC(Delta, ZSTD(1)),
			count Int64,
			size Int64
		) ENGINE MergeTree()
		ORDER BY (id);
		`,
		databaseName,
	)

	err := db.Exec(context.Background(), query)
	return err
}
