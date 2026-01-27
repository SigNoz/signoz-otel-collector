// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func Test_extractCountMetric(t *testing.T) {
	tests := []histogramTestCase{
		{
			name:         "histogram (non-monotonic)",
			input:        getTestHistogramMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(histogramMetric.Histogram().AggregationTemporality())
				countMetric.Sum().SetIsMonotonic(false)

				countMetric.SetName(histogramMetric.Name() + "_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(histogramMetric.Histogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "histogram (monotonic)",
			input:        getTestHistogramMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(histogramMetric.Histogram().AggregationTemporality())
				countMetric.Sum().SetIsMonotonic(true)

				countMetric.SetName(histogramMetric.Name() + "_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(histogramMetric.Histogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (non-monotonic)",
			input:        getTestExponentialHistogramMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(expHistogramMetric.ExponentialHistogram().AggregationTemporality())
				countMetric.Sum().SetIsMonotonic(false)

				countMetric.SetName(expHistogramMetric.Name() + "_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (monotonic)",
			input:        getTestExponentialHistogramMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(expHistogramMetric.ExponentialHistogram().AggregationTemporality())
				countMetric.Sum().SetIsMonotonic(true)

				countMetric.SetName(expHistogramMetric.Name() + "_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (non-monotonic)",
			input:        getTestSummaryMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				countMetric.Sum().SetIsMonotonic(false)

				countMetric.SetName("summary_metric_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (monotonic)",
			input:        getTestSummaryMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				countMetric := metrics.AppendEmpty()
				countMetric.SetEmptySum()
				countMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				countMetric.Sum().SetIsMonotonic(true)

				countMetric.SetName("summary_metric_count")
				dp := countMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "gauge (error)",
			input:        getTestGaugeMetric(),
			monotonicity: false,
			wantErr:      fmt.Errorf("extract_count_metric requires an input metric of type Histogram, ExponentialHistogram or Summary, got Gauge"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := extractCountMetric(tt.monotonicity)
			assert.NoError(t, err)

			resourceMetrics := pmetric.NewResourceMetrics()
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			tt.input.CopyTo(scopeMetrics.Metrics().AppendEmpty())
			metric := scopeMetrics.Metrics().At(0)
			ctx := ottlmetric.NewTransformContextPtr(resourceMetrics, scopeMetrics, metric)
			_, err = evaluate(nil, ctx)
			assert.Equal(t, tt.wantErr, err)

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)
				assert.Equal(t, expected, scopeMetrics.Metrics())
			}
		})
	}
}
