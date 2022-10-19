package clickhousetracesexporter

import (
	"strings"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opencensus.io/metric/metricdata"
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

func UsageExporter(metrics []*metricdata.Metric) (usage.Usage, error) {
	var u usage.Usage
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_spans_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_spans_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}
	return u, nil
}
