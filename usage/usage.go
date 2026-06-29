package usage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
	"go.uber.org/zap"
)

// Options provides options for LogExporter
type Options struct {
	// ReportingInterval is a time interval between two successive metrics export.
	ReportingInterval time.Duration
	// ReducedUsageTables, when non-empty, enables reporting of retained (reduced)
	// metric sample usage read from these ClickHouse tables. Only the metrics
	// exporter sets this (with reduction enabled); leaving it empty disables the
	// behavior, so logs/traces collectors never run it.
	ReducedUsageTables []string
}

type Usage struct {
	TimeStamp time.Time
	Count     int64
	Size      int64
}

type UsageCollector struct {
	exporterID           uuid.UUID
	reader               *metricexport.Reader
	ir                   *metricexport.IntervalReader
	initReaderOnce       sync.Once
	o                    Options
	db                   clickhouse.Conn
	dbName               string
	tableName            string
	distributedTableName string
	usageParser          func(metrics []*metricdata.Metric, exporterID uuid.UUID) (map[string]Usage, error)
	prevCount            int64
	prevSize             int64
	ttl                  int
	logger               *zap.Logger

	closeChan chan struct{}
	wg        sync.WaitGroup
}

var CollectorID uuid.UUID

func init() {
	CollectorID = uuid.New()
}

func NewUsageCollector(
	exporterId uuid.UUID,
	db clickhouse.Conn,
	options Options,
	dbName string,
	usageParser func(metrics []*metricdata.Metric, id uuid.UUID) (map[string]Usage, error),
	logger *zap.Logger,
) *UsageCollector {
	return &UsageCollector{
		exporterID:           exporterId,
		reader:               metricexport.NewReader(),
		o:                    options,
		db:                   db,
		dbName:               dbName,
		tableName:            UsageTableName,
		distributedTableName: "distributed_" + UsageTableName,
		usageParser:          usageParser,
		prevCount:            0,
		prevSize:             0,
		ttl:                  3,
		logger:               logger,
		closeChan:            make(chan struct{}),
	}
}

func (e *UsageCollector) Start() error {
	// start collector routine which
	e.initReaderOnce.Do(func() {
		var err error
		e.ir, err = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
		if err != nil {
			e.logger.Error("Error starting usage collector", zap.Error(err))
		}
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	if err := e.ir.Start(); err != nil {
		return err
	}
	// metrics-only: report retained (reduced) sample usage read from ClickHouse
	if len(e.o.ReducedUsageTables) > 0 {
		e.startReducedUsageReporter()
	}
	return nil
}

func (c *UsageCollector) Stop() error {
	c.ir.Stop()
	c.ir.Flush()
	close(c.closeChan)
	c.wg.Wait()
	return nil
}

func (e *UsageCollector) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	e.logger.Debug("ExportMetrics", zap.Any("db", e.db), zap.Any("dbName", e.dbName), zap.Any("distributedTableName", e.distributedTableName), zap.Any("metrics", metrics))
	usages, err := e.usageParser(metrics, e.exporterID)
	if err != nil {
		e.logger.Error("ExportMetrics parse error", zap.Error(err))
		return err
	}
	time := time.Now()
	for tenant, usage := range usages {
		usage.TimeStamp = time
		usageBytes, err := json.Marshal(usage)
		if err != nil {
			e.logger.Error("ExportMetrics marshal error", zap.Error(err))
			return err
		}
		encryptedData, err := Encrypt([]byte(e.exporterID.String())[:32], usageBytes)
		if err != nil {
			e.logger.Error("ExportMetrics encrypt error", zap.Error(err))
			return err
		}

		e.logger.Debug("ExportMetrics", zap.Any("tenant", tenant), zap.Any("collectorID", CollectorID.String()), zap.Any("exporterID", e.exporterID.String()), zap.Any("time", time), zap.Any("encryptedData", string(encryptedData)))
		// insert everything as a new row
		err = e.db.Exec(ctx, fmt.Sprintf("insert into %s.%s values ($1, $2, $3, $4, $5)", e.dbName, e.distributedTableName), tenant, CollectorID.String(), e.exporterID.String(), time, string(encryptedData))
		if err != nil {
			e.logger.Error("ExportMetrics insert error", zap.Error(err))
			return err
		}
	}
	return nil
}

const (
	reducedUsageSettleWindow = 15 * time.Minute
	reducedUsageTimeout      = 120 * time.Second
)

var reducedUsageNamespace = uuid.MustParse("6f9619ff-8b86-d011-b42d-00cf4fc964ff")

var reducedUsageCollectorID = uuid.MustParse("00000000-0000-0000-0000-7265647563ed")

func dayExporterID(day time.Time) uuid.UUID {
	return uuid.NewSHA1(reducedUsageNamespace, []byte("reduced-usage:"+day.UTC().Format("2006-01-02")))
}

func startOfUTCDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// startReducedUsageReporter periodically reports retained/reduced metric sample
// usage read from ClickHouse. Started only when Options.ReducedUsageTables is set.
func (e *UsageCollector) startReducedUsageReporter() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		report := func() {
			ctx, cancel := context.WithTimeout(context.Background(), reducedUsageTimeout)
			defer cancel()
			e.reportReducedUsage(ctx)
		}
		report() // first immediate report
		ticker := time.NewTicker(e.o.ReportingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-e.closeChan:
				return
			case <-ticker.C:
				report()
			}
		}
	}()
}

func (e *UsageCollector) reportReducedUsage(ctx context.Context) {
	now := time.Now().UTC()
	for _, day := range []time.Time{now.AddDate(0, 0, -1), now} {
		if err := e.writeReducedUsage(ctx, day, now); err != nil {
			e.logger.Error("failed to report reduced usage", zap.Time("day", day), zap.Error(err))
		}
	}
}

func (e *UsageCollector) writeReducedUsage(ctx context.Context, day, now time.Time) error {
	start := startOfUTCDay(day)
	end := start.Add(24 * time.Hour)
	if upper := now.Add(-reducedUsageSettleWindow); end.After(upper) {
		end = upper
	}
	if !end.After(start) {
		return nil // nothing settled for this day yet
	}

	count, err := e.reducedSampleCount(ctx, start, end)
	if err != nil {
		return err
	}

	exporterID := dayExporterID(day)
	payload, err := json.Marshal(Usage{TimeStamp: now, Count: int64(count)})
	if err != nil {
		return err
	}
	encrypted, err := Encrypt([]byte(exporterID.String())[:32], payload)
	if err != nil {
		return err
	}
	return e.db.Exec(ctx,
		fmt.Sprintf("insert into %s.%s values ($1, $2, $3, $4, $5)", e.dbName, e.distributedTableName),
		GetTenantNameFromResource(), reducedUsageCollectorID.String(), exporterID.String(), now, string(encrypted),
	)
}

// reducedSampleCount returns the number of distinct retained reduced samples in
// [start, end): one row per (env, temporality, metric_name, reduced_fingerprint,
// unix_milli), summed across the configured reduced tables.
func (e *UsageCollector) reducedSampleCount(ctx context.Context, start, end time.Time) (uint64, error) {
	var total uint64
	for _, table := range e.o.ReducedUsageTables {
		var count uint64
		query := fmt.Sprintf(`SELECT count() FROM (
			SELECT env, temporality, metric_name, reduced_fingerprint, unix_milli
			FROM %s.%s
			WHERE unix_milli >= ? AND unix_milli < ?
			GROUP BY env, temporality, metric_name, reduced_fingerprint, unix_milli
		)`, e.dbName, table)
		if err := e.db.QueryRow(ctx, query, start.UnixMilli(), end.UnixMilli()).Scan(&count); err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}
