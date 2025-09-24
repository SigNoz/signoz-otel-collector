// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/cwmetricstream"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

// resourceAttributes are the CloudWatch metric stream attributes that define a
// unique resource.
type resourceAttributes struct {
	// metricStreamName is the metric stream name.
	metricStreamName string
	// accountID is the AWS account ID.
	accountID string
	// region is the AWS region.
	region string
	// namespace is the CloudWatch metric namespace.
	namespace string
}

// The resourceMetricsBuilder is used to aggregate metrics for the
// same resourceAttributes.
type resourceMetricsBuilder struct {
	rms pmetric.MetricSlice
	// metricBuilders is the map of metrics within the same
	// resource group.
	metricBuilders map[string]*metricBuilder
}

// newResourceMetricsBuilder creates a resourceMetricsBuilder with the
// resourceAttributes.
func newResourceMetricsBuilder(md pmetric.Metrics, attrs resourceAttributes) *resourceMetricsBuilder {
	rms := md.ResourceMetrics().AppendEmpty()
	attrs.setAttributes(rms.Resource())
	return &resourceMetricsBuilder{
		rms:            rms.ScopeMetrics().AppendEmpty().Metrics(),
		metricBuilders: make(map[string]*metricBuilder),
	}
}

// AddMetric adds a metric to one of the metric builders based on
// the key generated for each.
func (rmb *resourceMetricsBuilder) AddMetric(metric cWMetric) {
	mb, ok := rmb.metricBuilders[metric.MetricName]
	if !ok {
		mb = newMetricBuilder(rmb.rms, metric.Namespace, metric.MetricName, metric.Unit)
		rmb.metricBuilders[metric.MetricName] = mb
	}
	mb.AddDataPoint(metric)
}

// setAttributes creates a pcommon.Resource from the fields in the resourceMetricsBuilder.
func (rmb *resourceAttributes) setAttributes(resource pcommon.Resource) {
	attributes := resource.Attributes()
	attributes.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attributes.PutStr(string(conventions.CloudAccountIDKey), rmb.accountID)
	attributes.PutStr(string(conventions.CloudRegionKey), rmb.region)
	serviceNamespace, serviceName := toServiceAttributes(rmb.namespace)
	if serviceNamespace != "" {
		attributes.PutStr(string(conventions.ServiceNamespaceKey), serviceNamespace)
	}
	attributes.PutStr(string(conventions.ServiceNameKey), serviceName)
	attributes.PutStr(attributeAWSCloudWatchMetricStreamName, rmb.metricStreamName)
}

// toServiceAttributes splits the CloudWatch namespace into service namespace/name
// if prepended by AWS/. Otherwise, it returns the CloudWatch namespace as the
// service name with an empty service namespace
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	index := strings.Index(namespace, namespaceDelimiter)
	if index != -1 && strings.EqualFold(namespace[:index], conventions.CloudProviderAWS.Value.AsString()) {
		return namespace[:index], namespace[index+1:]
	}
	return "", namespace
}

// dataPointKey combines the dimensions and timestamps to create a key
// used to prevent duplicate metrics.
type dataPointKey struct {
	// timestamp is the milliseconds since epoch
	timestamp int64
	// dimensions is the string representation of the metric dimensions.
	// fmt guarantees key-sorted order when printing a map.
	dimensions string
}

// The metricBuilder aggregates cwmetrics of the same name and unit
// into 1 gauge per value stat
type metricBuilder struct {
	metricNamespace string
	metricName      string
	unit            string

	resourceMetrics   pmetric.MetricSlice
	metricByValueStat map[string]pmetric.Metric
	// seen is the set of added data point keys.
	// to ensure duplicate data points are not repeated
	seen map[dataPointKey]bool
}

// newMetricBuilder creates a metricBuilder with the name and unit.
func newMetricBuilder(rms pmetric.MetricSlice, namespace, name, unit string) *metricBuilder {
	return &metricBuilder{
		metricNamespace:   namespace,
		metricName:        name,
		unit:              unit,
		resourceMetrics:   rms,
		metricByValueStat: map[string]pmetric.Metric{},
		seen:              make(map[dataPointKey]bool),
	}
}

// AddDataPoint adds the metric as a datapoint if a metric for that timestamp
// hasn't already been added.
func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	key := dataPointKey{
		timestamp:  metric.Timestamp,
		dimensions: fmt.Sprint(metric.Dimensions),
	}

	// only add if datapoint hasn't been seen already
	if _, ok := mb.seen[key]; !ok {
		mb.seen[key] = true

		for stat, val := range metric.statValues() {
			otlpMetric, exists := mb.metricByValueStat[stat]
			if !exists {
				otlpMetric = mb.resourceMetrics.AppendEmpty()
				otlpMetric.SetName(otlpMetricName(
					mb.metricNamespace, mb.metricName, stat,
				))
				otlpMetric.SetUnit(mb.unit)
				otlpMetric.SetEmptyGauge()

				mb.metricByValueStat[stat] = otlpMetric
			}

			dp := otlpMetric.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(val)
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
			for k, v := range metric.Dimensions {
				dp.Attributes().PutStr(ToSemConvAttributeKey(k), v)
			}
		}
	}
}

// ToSemConvAttributeKey maps some common keys to semantic convention attributes.
func ToSemConvAttributeKey(key string) string {
	switch key {
	case dimensionInstanceID:
		return string(conventions.ServiceInstanceIDKey)
	default:
		return key
	}
}

// make metrics more easily searchable.
func otlpMetricName(metricNamespace, metricName, statName string) string {
	// adding an aws_ prefix allows for quickly scoping down to aws metrics
	nameParts := []string{conventions.CloudProviderAWS.Value.AsString()}

	// including ns allows for distinguishing between metrics with same
	// name across multiple namespaces/services
	// For example, CPUUtilization is available for both EC2 and RDS
	nsParts := strings.Split(metricNamespace, namespaceDelimiter)
	for _, p := range nsParts {
		// ignore "AWS" in namespaces like "AWS/EC2" to avoid having
		// final names like aws_aws_ec2_CPUUtilization
		if strings.ToLower(p) != conventions.CloudProviderAWS.Value.AsString() && len(p) > 0 {
			nameParts = append(nameParts, p)
		}
	}

	nameParts = append(nameParts, metricName)

	if len(statName) > 0 {
		nameParts = append(nameParts, statName)
	}

	return strings.Join(nameParts, "_")
}

// helper for iterating over stats in cwMetric
func (m *cWMetric) statValues() map[string]float64 {
	return map[string]float64{
		"sum":   m.Value.Sum,
		"count": m.Value.Count,
		"min":   m.Value.Min,
		"max":   m.Value.Max,
	}
}
