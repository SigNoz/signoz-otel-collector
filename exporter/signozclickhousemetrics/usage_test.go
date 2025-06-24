package signozclickhousemetrics

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/metric/metricdata"
)

func TestUsageExporter(t *testing.T) {
	exporterId := uuid.New()
	tenant := "tenant1"

	labelKeys := []metricdata.LabelKey{
		{Key: usage.ExporterIDKey},
		{Key: usage.TenantKey},
	}
	labelValues := []metricdata.LabelValue{
		{Value: exporterId.String(), Present: true},
		{Value: tenant, Present: true},
	}

	tests := []struct {
		name    string
		metrics []*metricdata.Metric
		expect  map[string]usage.Usage
		hasErr  bool
	}{
		{
			name: "both count and size",
			metrics: []*metricdata.Metric{
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsCount, LabelKeys: labelKeys},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: labelValues, Points: []metricdata.Point{{Value: int64(42)}}}},
				},
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsBytes, LabelKeys: labelKeys},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: labelValues, Points: []metricdata.Point{{Value: int64(1000)}}}},
				},
			},
			expect: map[string]usage.Usage{tenant: {Count: 42, Size: 1000}},
			hasErr: false,
		},
		{
			name: "only count",
			metrics: []*metricdata.Metric{
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsCount, LabelKeys: labelKeys},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: labelValues, Points: []metricdata.Point{{Value: int64(7)}}}},
				},
			},
			expect: map[string]usage.Usage{tenant: {Count: 7}},
			hasErr: false,
		},
		{
			name: "only size",
			metrics: []*metricdata.Metric{
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsBytes, LabelKeys: labelKeys},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: labelValues, Points: []metricdata.Point{{Value: int64(555)}}}},
				},
			},
			expect: map[string]usage.Usage{tenant: {Size: 555}},
			hasErr: false,
		},
		{
			name: "missing required labels",
			metrics: []*metricdata.Metric{
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsCount, LabelKeys: []metricdata.LabelKey{}},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: []metricdata.LabelValue{}, Points: []metricdata.Point{{Value: int64(1)}}}},
				},
			},
			expect: nil,
			hasErr: true,
		},
		{
			name: "exporterId mismatch",
			metrics: []*metricdata.Metric{
				{
					Descriptor: metricdata.Descriptor{Name: SigNozMetricPointsCount, LabelKeys: labelKeys},
					TimeSeries: []*metricdata.TimeSeries{{LabelValues: []metricdata.LabelValue{{Value: uuid.New().String(), Present: true}, {Value: tenant, Present: true}}, Points: []metricdata.Point{{Value: int64(1)}}}},
				},
			},
			expect: map[string]usage.Usage{},
			hasErr: false,
		},
		{
			name:    "empty input",
			metrics: []*metricdata.Metric{},
			expect:  map[string]usage.Usage{},
			hasErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UsageExporter(tt.metrics, exporterId)
			if tt.hasErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expect, got)
			}
		})
	}
}
