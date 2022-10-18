package clickhouselogsexporter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const (
	SigNozSentLogRecordsKey      = "singoz_sent_log_records"
	SigNozSentLogRecordsBytesKey = "singoz_sent_log_records_bytes"
)

var (
	// Measures for usage
	ExporterSigNozSentLogRecords = stats.Int64(
		SigNozSentLogRecordsKey,
		"Number of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)
	ExporterSigNozSentLogRecordsBytes = stats.Int64(
		SigNozSentLogRecordsBytesKey,
		"Total size of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)

	// Views for usage
	LogsCountView = &view.View{
		Name:        "signoz_logs_count",
		Measure:     ExporterSigNozSentLogRecords,
		Description: "The number of logs exported to signoz",
		Aggregation: view.Sum(),
	}
	LogsSizeView = &view.View{
		Name:        "signoz_logs_bytes",
		Measure:     ExporterSigNozSentLogRecordsBytes,
		Description: "The size of logs exported to signoz",
		Aggregation: view.Sum(),
	}
)

// ClickhouseMetricsExporter exports usage data to clickhouse
type ClickhouseLogsUsageExporter struct {
	id             string
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              usage.Options
	db             clickhouse.Conn
}

// NewExporter creates new clickhouse usage exporter.
func NewExporter(db clickhouse.Conn, options usage.Options) (*ClickhouseLogsUsageExporter, error) {
	e := &ClickhouseLogsUsageExporter{
		id:     uuid.NewString(),
		reader: metricexport.NewReader(),
		o:      options,
		db:     db,
	}

	err := usage.Init(db, "signoz_logs")
	if err != nil {
		return nil, err
	}

	return e, nil
}

// Start starts the metric and span data exporter.
func (e *ClickhouseLogsUsageExporter) Start() error {
	e.initReaderOnce.Do(func() {
		e.ir, _ = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	return e.ir.Start()
}

// Stop stops the metric exporter.
func (e *ClickhouseLogsUsageExporter) Stop() {
	e.ir.Stop()
}

// Close closes anything that is open
func (e *ClickhouseLogsUsageExporter) Close() {
}

// ExportMetrics exports to clickhouse.
func (e *ClickhouseLogsUsageExporter) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	var count, size int64
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_logs_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_logs_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}

	if count == 0 && size == 0 {
		return nil
	}

	usages := []usage.Usage{}
	err := e.db.Select(ctx, &usages, fmt.Sprintf("SELECT * FROM signoz_logs.usage where id='%s'", e.id))
	if err != nil {
		return err
	}

	if len(usages) > 0 {
		// already exist then update
		err := e.db.Exec(ctx, "alter table signoz_logs.usage update datetime = $1, count = $2, size = $3 where id = $4", time.Now(), count, size, e.id)
		if err != nil {
			return err
		}
	} else {
		err := e.db.Exec(ctx, "insert into signoz_logs.usage values ($1, $2, $3, $4)", e.id, time.Now(), count, size)
		if err != nil {
			return err
		}
	}

	return nil
}
