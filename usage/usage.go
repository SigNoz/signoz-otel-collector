package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
)

// Options provides options for LogExporter
type Options struct {
	// ReportingInterval is a time interval between two successive metrics export.
	ReportingInterval time.Duration
}

type Usage struct {
	TimeStamp time.Time
	Count     int64
	Size      int64
}

type UsageCollector struct {
	id             uuid.UUID
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              Options
	db             clickhouse.Conn
	dbName         string
	tableName      string
	usageParser    func(metrics []*metricdata.Metric) (map[string]Usage, error)
	prevCount      int64
	prevSize       int64
}

var CollectorID uuid.UUID

func init() {
	CollectorID = uuid.New()
}

func NewUsageCollector(db clickhouse.Conn, options Options, dbName string, usageParser func(metrics []*metricdata.Metric) (map[string]Usage, error)) *UsageCollector {
	return &UsageCollector{
		id:          uuid.New(),
		reader:      metricexport.NewReader(),
		o:           options,
		db:          db,
		dbName:      dbName,
		tableName:   "usage",
		usageParser: usageParser,
		prevCount:   0,
		prevSize:    0,
	}
}
func (e *UsageCollector) CreateTable(db clickhouse.Conn, databaseName string) error {
	query := fmt.Sprintf(
		`
		CREATE TABLE IF NOT EXISTS %s.usage (
			collector_id String,
			exporter_id String,
			timestamp DateTime,
			tenant String,
			data String
		) ENGINE MergeTree()
		ORDER BY (collector_id, exporter_id, tenant);
		`,
		databaseName,
	)

	err := db.Exec(context.Background(), query)
	return err
}

func (e *UsageCollector) Start() error {
	// create table if not exists
	err := e.CreateTable(e.db, e.dbName)
	if err != nil {
		return err
	}

	// start collector routine which
	e.initReaderOnce.Do(func() {
		e.ir, _ = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	return e.ir.Start()
}

func (c *UsageCollector) Stop() error {
	c.ir.Stop()
	return nil
}

func (e *UsageCollector) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	usages, err := e.usageParser(metrics)
	if err != nil {
		return err
	}
	time := time.Now()
	for tenant, usage := range usages {
		usage.TimeStamp = time
		usageBytes, err := json.Marshal(usage)
		if err != nil {
			return err
		}
		encryptedData, err := Encrypt([]byte(e.id.String())[:32], usageBytes)
		if err != nil {
			return err
		}
		prevUsages := []UsageDB{}
		err = e.db.Select(ctx, &prevUsages, fmt.Sprintf("SELECT * FROM %s.usage where collector_id='%s' and exporter_id='%s' and tenant='%s'", e.dbName, CollectorID.String(), e.id.String(), tenant))
		if err != nil {
			return err
		}
		if len(prevUsages) > 0 {
			// already exist then update
			err := e.db.Exec(ctx, fmt.Sprintf("alter table %s.usage update timestamp=$1, data=$2 where collector_id=$3 and exporter_id=$4 and tenant=$5", e.dbName), time, string(encryptedData), CollectorID.String(), e.id.String(), tenant)
			if err != nil {
				return err
			}
		} else {
			err := e.db.Exec(ctx, fmt.Sprintf("insert into %s.usage values ($1, $2, $3, $4, $5)", e.dbName), CollectorID.String(), e.id.String(), time, tenant, string(encryptedData))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
