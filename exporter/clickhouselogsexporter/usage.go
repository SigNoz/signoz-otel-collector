package clickhouselogsexporter

import (
	"fmt"
	"maps"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	SigNozSentLogRecordsKey      = "singoz_sent_log_records"
	SigNozSentLogRecordsBytesKey = "singoz_sent_log_records_bytes"
	SigNozLogsCount              = "signoz_logs_count"
	SigNozLogsBytes              = "signoz_logs_bytes"
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
		Name:        SigNozLogsCount,
		Measure:     ExporterSigNozSentLogRecords,
		Description: "The number of logs exported to signoz",
		Aggregation: view.Sum(),
		TagKeys:     []tag.Key{usage.TagTenantKey, usage.TagExporterIdKey},
	}
	LogsSizeView = &view.View{
		Name:        SigNozLogsBytes,
		Measure:     ExporterSigNozSentLogRecordsBytes,
		Description: "The size of logs exported to signoz",
		Aggregation: view.Sum(),
		TagKeys:     []tag.Key{usage.TagTenantKey, usage.TagExporterIdKey},
	}
)

func UsageExporter(metrics []*metricdata.Metric, id uuid.UUID) (map[string]usage.Usage, error) {
	data := map[string]usage.Usage{}
	for _, metric := range metrics {
		if !strings.Contains(metric.Descriptor.Name, SigNozLogsCount) && !strings.Contains(metric.Descriptor.Name, SigNozLogsBytes) {
			continue
		}
		exporterIndex := usage.GetIndexOfLabel(metric.Descriptor.LabelKeys, usage.ExporterIDKey)
		tenantIndex := usage.GetIndexOfLabel(metric.Descriptor.LabelKeys, usage.TenantKey)
		if exporterIndex == -1 || tenantIndex == -1 {
			return nil, fmt.Errorf("usage: failed to get index of labels")
		}

		if strings.Contains(metric.Descriptor.Name, SigNozLogsCount) {
			for _, v := range metric.TimeSeries {
				if v.LabelValues[exporterIndex].Value != id.String() {
					continue
				}
				tenant := v.LabelValues[tenantIndex].Value
				if d, ok := data[tenant]; ok {
					d.Count = v.Points[0].Value.(int64)
					data[tenant] = d
				} else {
					data[tenant] = usage.Usage{
						Count: v.Points[0].Value.(int64),
					}
				}
			}
		} else if strings.Contains(metric.Descriptor.Name, SigNozLogsBytes) {
			for _, v := range metric.TimeSeries {
				if v.LabelValues[exporterIndex].Value != id.String() {
					continue
				}
				tenant := v.LabelValues[tenantIndex].Value
				if d, ok := data[tenant]; ok {
					d.Size = v.Points[0].Value.(int64)
					data[tenant] = d
				} else {
					data[tenant] = usage.Usage{
						Size: v.Points[0].Value.(int64),
					}
				}
			}
		}
	}
	return data, nil
}

// isBillableAttribute must agree with the exclude patterns of the logs meter in pkg/metering/v1.
func isBillableAttribute(key string) bool {
	// Every exclude pattern starts with "signoz"; skip the regexes for all other keys.
	if !strings.HasPrefix(key, "signoz") {
		return true
	}
	return !metering.ExcludeSigNozWorkspaceResourceAttrs.MatchString(key) && !isInternalAttr(key)
}

// billableAttributesSize returns the billed size of attrs, whose stored JSON is attributesJSON.
func billableAttributesSize(attrs map[string]any, attributesJSON string) int {
	var billable map[string]any
	for k := range attrs {
		if isBillableAttribute(k) {
			continue
		}
		if billable == nil {
			billable = maps.Clone(attrs)
		}
		delete(billable, k)
	}
	if billable == nil {
		return len(attributesJSON)
	}
	if len(billable) == 0 {
		return len("{}")
	}
	b, err := json.Marshal(billable)
	if err != nil {
		return len(attributesJSON)
	}
	return len(b)
}

func getResourceAttributesByte(resource pcommon.Resource) ([]byte, error) {
	filteredResources := map[string]any{}
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		if isBillableAttribute(k) {
			filteredResources[k] = v.AsRaw()
		}
		return true
	})
	resBytes, err := json.Marshal(filteredResources)
	if err != nil {
		return nil, err
	}

	return resBytes, nil
}
