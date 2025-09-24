// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

const (
	testRegion     = "us-east-1"
	testAccountID  = "1234567890"
	testStreamName = "MyMetricStream"
	testInstanceID = "i-1234567890abcdef0"
)

func TestToSemConvAttributeKey(t *testing.T) {
	testCases := map[string]struct {
		key  string
		want string
	}{
		"WithValidKey": {
			key:  "InstanceId",
			want: string(conventions.ServiceInstanceIDKey),
		},
		"WithInvalidKey": {
			key:  "CustomDimension",
			want: "CustomDimension",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := ToSemConvAttributeKey(testCase.key)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestMetricBuilder(t *testing.T) {
	t.Run("WithSingleMetric", func(t *testing.T) {
		metric := cWMetric{
			MetricName: "name",
			Unit:       "unit",
			Timestamp:  time.Now().UnixMilli(),
			Value:      testCWMetricValue(),
			Dimensions: map[string]string{"CustomDimension": "test"},
		}
		gots := pmetric.NewMetricSlice()
		mb := newMetricBuilder(gots, metric.Namespace, metric.MetricName, metric.Unit)
		mb.AddDataPoint(metric)
		require.Equal(t, 4, gots.Len())

		for stat, expectedValue := range metric.statValues() {
			expectedName := otlpMetricName(metric.Namespace, metric.MetricName, stat)
			found := findMetricByName(t, gots, expectedName)
			require.Equal(t, expectedName, found.Name())

			require.Equal(t, metric.Unit, found.Unit())

			require.Equal(t, pmetric.MetricTypeGauge, found.Type())

			foundDps := found.Gauge().DataPoints()
			require.Equal(t, 1, foundDps.Len())
			foundDp := foundDps.At(0)
			require.Equal(t, expectedValue, foundDp.DoubleValue())
			require.Equal(t, 1, foundDp.Attributes().Len())
		}

	})

	t.Run("WithTimestampCollision", func(t *testing.T) {
		// all but the first cwMetric should get ignored
		timestamp := time.Now().UnixMilli()
		metrics := []cWMetric{
			{
				Namespace:  "namespace",
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  timestamp,
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{
					"AccountId":  testAccountID,
					"Region":     testRegion,
					"InstanceId": testInstanceID,
				},
			},
			{
				Namespace:  "namespace",
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  timestamp,
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{
					"InstanceId": testInstanceID,
					"AccountId":  testAccountID,
					"Region":     testRegion,
				},
			},
		}
		gots := pmetric.NewMetricSlice()
		mb := newMetricBuilder(gots, "namespace", "name", "unit")
		for _, metric := range metrics {
			mb.AddDataPoint(metric)
		}
		require.Equal(t, 4, gots.Len())

		expectedIncludedMetric := metrics[0]

		for stat, expectedValue := range expectedIncludedMetric.statValues() {
			expectedName := otlpMetricName(
				expectedIncludedMetric.Namespace, expectedIncludedMetric.MetricName, stat,
			)
			found := findMetricByName(t, gots, expectedName)
			require.Equal(t, expectedName, found.Name())

			require.Equal(t, expectedIncludedMetric.Unit, found.Unit())

			require.Equal(t, pmetric.MetricTypeGauge, found.Type())

			foundDps := found.Gauge().DataPoints()
			require.Equal(t, 1, foundDps.Len())
			foundDp := foundDps.At(0)
			require.Equal(t, expectedValue, foundDp.DoubleValue())
			require.Equal(t, 3, foundDp.Attributes().Len())
		}
	})
}

func TestResourceMetricsBuilder(t *testing.T) {
	testCases := map[string]struct {
		namespace      string
		wantAttributes map[string]string
	}{
		"WithAwsNamespace": {
			namespace: "AWS/EC2",
			wantAttributes: map[string]string{
				attributeAWSCloudWatchMetricStreamName:  testStreamName,
				string(conventions.CloudAccountIDKey):   testAccountID,
				string(conventions.CloudRegionKey):      testRegion,
				string(conventions.ServiceNameKey):      "EC2",
				string(conventions.ServiceNamespaceKey): "AWS",
			},
		},
		"WithCustomNamespace": {
			namespace: "CustomNamespace",
			wantAttributes: map[string]string{
				attributeAWSCloudWatchMetricStreamName:  testStreamName,
				string(conventions.CloudAccountIDKey):   testAccountID,
				string(conventions.CloudRegionKey):      testRegion,
				string(conventions.ServiceNameKey):      "CustomNamespace",
				string(conventions.ServiceNamespaceKey): "",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metric := cWMetric{
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			}
			attrs := resourceAttributes{
				metricStreamName: testStreamName,
				accountID:        testAccountID,
				region:           testRegion,
				namespace:        testCase.namespace,
			}
			gots := pmetric.NewMetrics()
			rmb := newResourceMetricsBuilder(gots, attrs)
			rmb.AddMetric(metric)
			require.Equal(t, 1, gots.ResourceMetrics().Len())
			got := gots.ResourceMetrics().At(0)
			gotAttrs := got.Resource().Attributes()
			for wantKey, wantValue := range testCase.wantAttributes {
				gotValue, ok := gotAttrs.Get(wantKey)
				if wantValue != "" {
					require.True(t, ok)
					require.Equal(t, wantValue, gotValue.AsString())
				} else {
					require.False(t, ok)
				}
			}
		})
	}
	t.Run("WithSameMetricDifferentDimensions", func(t *testing.T) {
		metrics := []cWMetric{
			{
				Namespace:  "AWS/EC2",
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			},
			{
				Namespace:  "AWS/EC2",
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().Add(time.Second * 3).UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{
					"CustomDimension": "value",
				},
			},
		}
		attrs := resourceAttributes{
			metricStreamName: testStreamName,
			accountID:        testAccountID,
			region:           testRegion,
			namespace:        "AWS/EC2",
		}
		gots := pmetric.NewMetrics()
		rmb := newResourceMetricsBuilder(gots, attrs)
		for _, metric := range metrics {
			rmb.AddMetric(metric)
		}
		require.Equal(t, 1, gots.ResourceMetrics().Len())
		got := gots.ResourceMetrics().At(0)
		require.Equal(t, 1, got.ScopeMetrics().Len())
		gotMetrics := got.ScopeMetrics().At(0).Metrics()
		require.Equal(t, 4, gotMetrics.Len())

		for i := 0; i < gotMetrics.Len(); i++ {
			gotDps := gotMetrics.At(i).Gauge().DataPoints()
			require.Equal(t, 2, gotDps.Len())
		}
	})
}

// testCWMetricValue is a convenience function for creating a test cWMetricValue
func testCWMetricValue() *cWMetricValue {
	return &cWMetricValue{100, 0, float64(rand.Int63n(100)), float64(rand.Int63n(4))}
}

func TestToOtlpMetricName(t *testing.T) {
	testCases := map[string]struct {
		namespace string
		name      string
		stat      string
		want      string
	}{
		"WithAWSNamespace": {
			namespace: "AWS/EC2",
			name:      "CPUUtilization",
			stat:      "sum",
			want:      "aws_EC2_CPUUtilization_sum",
		},
		"WithCustomNamespace": {
			namespace: "EKS/NODE",
			name:      "CPUUtilization",
			stat:      "sum",
			want:      "aws_EKS_NODE_CPUUtilization_sum",
		},
		"WithoutNamespace": {
			namespace: "",
			name:      "CPUUtilization",
			stat:      "sum",
			want:      "aws_CPUUtilization_sum",
		},
		"WithoutStat": {
			namespace: "AWS/EC2",
			name:      "CPUUtilization",
			stat:      "",
			want:      "aws_EC2_CPUUtilization",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := otlpMetricName(testCase.namespace, testCase.name, testCase.stat)
			require.Equal(t, testCase.want, got)
		})
	}
}

// test helper
func findMetricByName(t *testing.T, ms pmetric.MetricSlice, name string) pmetric.Metric {
	matches := []pmetric.Metric{}
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Name() == name {
			matches = append(matches, ms.At(i))
		}
	}

	require.Equal(
		t, len(matches), 1,
		fmt.Sprintf("expected metric with name %s not found", name),
	)

	return matches[0]
}
