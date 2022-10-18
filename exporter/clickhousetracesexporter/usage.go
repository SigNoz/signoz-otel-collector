package clickhousetracesexporter

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
	SigNozSentSpansKey      = "singoz_sent_spans"
	SigNozSentSpansBytesKey = "singoz_sent_spans_bytes"
)

var (
	// Measures for usage
	ExporterSigNozSentSpans = stats.Int64(
		SigNozSentSpansKey,
		"Number of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)
	ExporterSigNozSentSpansBytes = stats.Int64(
		SigNozSentSpansBytesKey,
		"Total size of signoz log records successfully sent to destination.",
		stats.UnitDimensionless)

	// Views for usage
	SpansCountView = &view.View{
		Name:        "signoz_spans_count",
		Measure:     ExporterSigNozSentSpans,
		Description: "The number of spans exported to signoz",
		Aggregation: view.Sum(),
	}
	SpansCountBytesView = &view.View{
		Name:        "signoz_spans_bytes",
		Measure:     ExporterSigNozSentSpansBytes,
		Description: "The size of spans exported to signoz",
		Aggregation: view.Sum(),
	}
)

// ClickhouseMetricsExporter exports usage data to clickhouse
type ClickhouseSpansUsageExporter struct {
	id             string
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              usage.Options
	db             clickhouse.Conn
}

// NewExporter creates new clickhouse usage exporter.
func NewExporter(db clickhouse.Conn, options usage.Options) (*ClickhouseSpansUsageExporter, error) {
	e := &ClickhouseSpansUsageExporter{
		id:     uuid.NewString(),
		reader: metricexport.NewReader(),
		o:      options,
		db:     db,
	}

	err := usage.Init(db, "signoz_traces")
	if err != nil {
		return nil, err
	}

	return e, nil
}

// Start starts the metric and span data exporter.
func (e *ClickhouseSpansUsageExporter) Start() error {
	e.initReaderOnce.Do(func() {
		e.ir, _ = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	return e.ir.Start()
}

// Stop stops the metric exporter.
func (e *ClickhouseSpansUsageExporter) Stop() {
	e.ir.Stop()
}

// Close closes any files that were opened for logging.
func (e *ClickhouseSpansUsageExporter) Close() {

}

// ExportMetrics exports to clickhouse.
func (e *ClickhouseSpansUsageExporter) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	var count, size int64
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_spans_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_spans_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}

	if count == 0 && size == 0 {
		return nil
	}

	usages := []usage.Usage{}
	err := e.db.Select(ctx, &usages, fmt.Sprintf("SELECT * FROM signoz_traces.usage where id='%s'", e.id))
	if err != nil {
		return err
	}

	if len(usages) > 0 {
		// already exist then update
		err := e.db.Exec(ctx, "alter table signoz_traces.usage update datetime = $1, count = $2, size = $3 where id = $4", time.Now(), count, size, e.id)
		if err != nil {
			return err
		}
	} else {
		err := e.db.Exec(ctx, "insert into signoz_traces.usage values ($1, $2, $3, $4)", e.id, time.Now(), count, size)
		if err != nil {
			return err
		}
	}

	return nil
}
