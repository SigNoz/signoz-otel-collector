package clickhousemetricsexporter

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

// ExportMetrics exports to log.
func (e *ClickhouseMetricsUsageExporter) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_metric") {
			for _, ts := range metric.TimeSeries {
				for _, point := range ts.Points {
					fmt.Println(e.id, metric.Descriptor.Name, point.Value)
				}
			}
		}
	}
	return nil
}
