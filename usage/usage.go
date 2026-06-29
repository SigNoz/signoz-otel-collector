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

const (
	// 10m refreshable mvs + 5 minutes grace
	reducedUsageSettleWindow = 15 * time.Minute
	reducedUsageTimeout      = 120 * time.Second
	// report every hour window
	reducedUsageWindow = time.Hour
	// grace look back
	reducedUsageLookbackWindows = 3
)

var reducedUsageNamespace = uuid.MustParse("6f9619ff-8b86-d011-b42d-00cf4fc964ff")

var reducedUsageCollectorID = uuid.MustParse("00000000-0000-0000-0000-7265647563ed")

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

	closeChan     chan struct{}
	wg            sync.WaitGroup
	reducedCancel context.CancelFunc
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
	if len(e.o.ReducedUsageTables) > 0 {
		e.startReducedUsageReporter()
	}
	return nil
}

func (c *UsageCollector) Stop() error {
	c.ir.Stop()
	c.ir.Flush()
	// cancel any in-flight reduced-usage query so shutdown isn't blocked
	// then stop the reporter loop and wait for it to exit
	// before the caller closes the shared connection.
	if c.reducedCancel != nil {
		c.reducedCancel()
	}
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

func windowStartOf(t time.Time) time.Time {
	return t.UTC().Truncate(reducedUsageWindow)
}

func windowExporterID(windowStart time.Time) uuid.UUID {
	return uuid.NewSHA1(reducedUsageNamespace, []byte(fmt.Sprintf("reduced-usage:%d", windowStart.UTC().Unix())))
}

// startReducedUsageReporter periodically reports retained/reduced metric sample
// usage read from ClickHouse. Started only when Options.ReducedUsageTables is set.
func (e *UsageCollector) startReducedUsageReporter() {
	ctx, cancel := context.WithCancel(context.Background())
	e.reducedCancel = cancel
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer cancel()
		report := func() {
			rctx, rcancel := context.WithTimeout(ctx, reducedUsageTimeout)
			defer rcancel()
			e.reportReducedUsage(rctx)
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
	e.reportReducedUsageAt(ctx, time.Now().UTC())
}

// reportReducedUsageAt writes the last few finished hour windows. A window is
// written only after its data is final, and its count never changes after that,
// so writing the same window again on later ticks does not affect anything
func (e *UsageCollector) reportReducedUsageAt(ctx context.Context, now time.Time) {
	for _, windowStart := range reducedWindowStarts(now) {
		if err := e.writeReducedWindow(ctx, windowStart); err != nil {
			e.logger.Error("failed to report reduced usage", zap.Time("window_start", windowStart), zap.Error(err))
		}
	}
}

// reducedWindowStarts returns the start times of the last few finished hour
// windows, newest first. An hour's data is final once the hour ended at least
// reducedUsageSettleWindow ago, so we go back from (now - settle).
func reducedWindowStarts(now time.Time) []time.Time {
	readyUntil := now.Add(-reducedUsageSettleWindow)
	lastStart := windowStartOf(readyUntil).Add(-reducedUsageWindow)
	starts := make([]time.Time, reducedUsageLookbackWindows)
	for i := range starts {
		starts[i] = lastStart.Add(-time.Duration(i) * reducedUsageWindow)
	}
	return starts
}

// writeReducedWindow counts the retained data points in one finished hour
// window and writes a single row for it. The row is stamped at the window's start
// so latest wins during the insert and uses a per-window id, so every pod writes
// the exact same row (duplicates collapse on read).
// the count is associated to the window's own day
func (e *UsageCollector) writeReducedWindow(ctx context.Context, windowStart time.Time) error {
	windowEnd := windowStart.Add(reducedUsageWindow)

	count, err := e.reducedSampleCount(ctx, windowStart, windowEnd)
	if err != nil {
		return err
	}

	exporterID := windowExporterID(windowStart)
	payload, err := json.Marshal(Usage{TimeStamp: windowStart, Count: int64(count)})
	if err != nil {
		return err
	}
	encrypted, err := Encrypt([]byte(exporterID.String())[:32], payload)
	if err != nil {
		return err
	}
	return e.db.Exec(ctx,
		fmt.Sprintf("insert into %s.%s values ($1, $2, $3, $4, $5)", e.dbName, e.distributedTableName),
		GetTenantNameFromResource(), reducedUsageCollectorID.String(), exporterID.String(), windowStart, string(encrypted),
	)
}

func (e *UsageCollector) reducedSampleCount(ctx context.Context, start, end time.Time) (uint64, error) {
	var total uint64
	for _, table := range e.o.ReducedUsageTables {
		var count uint64
		// The fingerprint is built from env + temporality + metric_name, so
		// (reduced_fingerprint, unix_milli) identifies a row uniquely.
		// Grouping by just these two columns reads less data and uses far less
		// memory than grouping by all five (measured ~3.7x faster, ~2.7x less memory
		// on the cluster). It still counts correctly because every row for a
		// fingerprint sits on one node. max_threads keeps this background query
		// from taking too much of the database at once.
		query := fmt.Sprintf(`SELECT count() FROM (
			SELECT reduced_fingerprint, unix_milli
			FROM %s.%s
			WHERE unix_milli >= ? AND unix_milli < ?
			GROUP BY reduced_fingerprint, unix_milli
		) SETTINGS max_threads = 8`, e.dbName, table)
		if err := e.db.QueryRow(ctx, query, start.UnixMilli(), end.UnixMilli()).Scan(&count); err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}
