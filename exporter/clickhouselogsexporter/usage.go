package clickhouselogsexporter

import (
	"strings"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opencensus.io/metric/metricdata"
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

func UsageExporter(metrics []*metricdata.Metric) (usage.Usage, error) {
	var u usage.Usage
	for _, metric := range metrics {
		if strings.Contains(metric.Descriptor.Name, "signoz_logs_count") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Count = metric.TimeSeries[0].Points[0].Value.(int64)
		} else if strings.Contains(metric.Descriptor.Name, "signoz_logs_bytes") && len(metric.TimeSeries) == 1 && len(metric.TimeSeries[0].Points) == 1 {
			u.Size = metric.TimeSeries[0].Points[0].Value.(int64)
		}
	}
	return u, nil
}
