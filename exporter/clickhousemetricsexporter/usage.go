package clickhousemetricsexporter

import (
	"strings"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opencensus.io/metric/metricdata"
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

func UsageExporter(metrics []*metricdata.Metric) (usage.Usage, error) {
	var u usage.Usage
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_metric_points_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_metric_points_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}
	return u, nil
}
