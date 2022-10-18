package clickhousemetricsexporter

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
	SigNozSentMetricPointsKey      = "singoz_sent_metric_points"
	SigNozSentMetricPointsBytesKey = "singoz_sent_metric_points_bytes"
)

var (
	// Measures for usage
	ExporterSigNozSentMetricPoints = stats.Int64(
		SigNozSentMetricPointsKey,
		"Number of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)
	ExporterSigNozSentMetricPointsBytes = stats.Int64(
		SigNozSentMetricPointsBytesKey,
		"Total size of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)

	// Views for usage
	MetricPointsCountView = &view.View{
		Name:        "signoz_metric_points_count",
		Measure:     ExporterSigNozSentMetricPoints,
		Description: "The number of logs exported to signoz",
		Aggregation: view.Sum(),
	}
	MetricPointsBytesView = &view.View{
		Name:        "signoz_metric_points_bytes",
		Measure:     ExporterSigNozSentMetricPointsBytes,
		Description: "The size of logs exported to signoz",
		Aggregation: view.Sum(),
	}
)

// ClickhouseMetricsExporter exports usage data to clickhouse
type ClickhouseMetricsUsageExporter struct {
	id             string
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              usage.Options
	db             clickhouse.Conn
}

// NewExporter creates new clickhouse usage exporter.
func NewExporter(db clickhouse.Conn, options usage.Options) (*ClickhouseMetricsUsageExporter, error) {
	e := &ClickhouseMetricsUsageExporter{
		id:     uuid.NewString(),
		reader: metricexport.NewReader(),
		o:      options,
		db:     db,
	}

	// create the table if not exist
	err := usage.Init(db, "signoz_metrics")
	if err != nil {
		return nil, err
	}

	return e, nil
}

// Start starts the metric and span data exporter.
func (e *ClickhouseMetricsUsageExporter) Start() error {
	e.initReaderOnce.Do(func() {
		e.ir, _ = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	return e.ir.Start()
}

// Stop stops the metric exporter.
func (e *ClickhouseMetricsUsageExporter) Stop() {
	e.ir.Stop()
}

// Close closes any files that were opened for logging.
func (e *ClickhouseMetricsUsageExporter) Close() {

}

// ExportMetrics exports to clickhouse.
func (e *ClickhouseMetricsUsageExporter) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	var count, size int64
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_metric_points_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_metric_points_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}

	if count == 0 && size == 0 {
		return nil
	}

	usages := []usage.Usage{}
	err := e.db.Select(ctx, &usages, fmt.Sprintf("SELECT * FROM signoz_metrics.usage where id='%s'", e.id))
	if err != nil {
		return err
	}

	if len(usages) > 0 {
		// already exist then update
		err := e.db.Exec(ctx, "alter table signoz_metrics.usage update datetime = $1, count = $2, size = $3 where id = $4", time.Now(), count, size, e.id)
		if err != nil {
			return err
		}
	} else {
		err := e.db.Exec(ctx, "insert into signoz_metrics.usage values ($1, $2, $3, $4)", e.id, time.Now(), count, size)
		if err != nil {
			return err
		}
	}

	return nil
}
