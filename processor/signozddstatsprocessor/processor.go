// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozddstatsprocessor

import (
	"context"
	"fmt"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	attrService    = "service"
	attrResource   = "resource"
	attrStatusCode = "status_code"
	attrType       = "type"
	attrSpanKind   = "span.kind"

	ddStatsPayloadMetric = "dd.internal.stats.payload"
)

type ddStatsProcessor struct {
	logger *zap.Logger
	config *Config
}

func newProcessor(set processor.Settings, cfg *Config) *ddStatsProcessor {
	return &ddStatsProcessor{
		logger: set.Logger,
		config: cfg,
	}
}

func (p *ddStatsProcessor) ProcessMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !p.config.Enabled {
		return md, nil
	}

	outMetrics := pmetric.NewMetrics()
	outResourceMetrics := outMetrics.ResourceMetrics()

	resourceMetricsSlice := md.ResourceMetrics()
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		scopeMetricsSlice := rm.ScopeMetrics()

		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			sm := scopeMetricsSlice.At(j)
			metricsSlice := sm.Metrics()

			for k := 0; k < metricsSlice.Len(); k++ {
				metric := metricsSlice.At(k)

				// Process only dd.internal.stats.payload metrics
				if metric.Name() == ddStatsPayloadMetric {
					if err := p.processStatsPayload(metric, rm.Resource(), &outMetrics); err != nil {
						p.logger.Error("failed to process DD stats payload", zap.Error(err))
						// Continue processing other metrics
						continue
					}
				} else {
					// Pass through other metrics unchanged TODO: Optimize to avoid copying if no dd stats metrics are present
					outRM := outResourceMetrics.AppendEmpty()
					rm.Resource().CopyTo(outRM.Resource())
					outSM := outRM.ScopeMetrics().AppendEmpty()
					sm.Scope().CopyTo(outSM.Scope())
					outMetric := outSM.Metrics().AppendEmpty()
					metric.CopyTo(outMetric)
				}
			}
		}
	}

	return outMetrics, nil
}

func (p *ddStatsProcessor) processStatsPayload(metric pmetric.Metric, resource pcommon.Resource, outMetrics *pmetric.Metrics) error {

	sum := metric.Sum()
	dataPoints := sum.DataPoints()

	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		// Get the payload from attributes
		payloadAttr, exists := dp.Attributes().Get("dd.internal.stats.payload")
		if !exists {
			p.logger.Warn("payload attribute not found in DD stats metric")
			continue
		}

		// // Decode base64 payload
		// payloadBytes, err := base64.StdEncoding.DecodeString(payloadAttr.Str())
		// if err != nil {
		// 	return fmt.Errorf("failed to decode payload: %w", err)
		// }

		// // Unmarshal protobuf
		// var statsPayload pb.StatsPayload
		// if err := proto.Unmarshal(payloadBytes, &statsPayload); err != nil {
		// 	return fmt.Errorf("failed to unmarshal stats payload: %w", err)
		// }

		statsPayload, err := p.decodePayload(payloadAttr.Bytes().AsRaw())
		if err != nil {
			p.logger.Error("failed to decode dd.internal.stats.payload",
				zap.Error(err))
			continue
		}

		// Process the stats payload and generate OTLP metrics
		if err := p.generateOTLPMetrics(statsPayload, resource, outMetrics); err != nil {
			return fmt.Errorf("failed to generate OTLP metrics: %w", err)
		}
	}

	return nil
}

// decodePayload decodes protobuf payload from raw bytes
func (p *ddStatsProcessor) decodePayload(data []byte) (*pb.StatsPayload, error) {
	p.logger.Debug("Attempting to decode payload", zap.Int("payload_size", len(data)))

	// If that fails, try as StatsPayload
	var statsPayload pb.StatsPayload
	if err := proto.Unmarshal(data, &statsPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal as both ClientStatsPayload and StatsPayload: %w", err)
	}

	// Validate we have stats
	if len(statsPayload.Stats) == 0 {
		return nil, fmt.Errorf("StatsPayload contains no ClientStatsPayload")
	}

	p.logger.Debug("Successfully decoded as StatsPayload",
		zap.Int("client_stats_count", len(statsPayload.Stats)),
		zap.Bool("client_computed", statsPayload.ClientComputed))

	return &statsPayload, nil
}

func (p *ddStatsProcessor) generateOTLPMetrics(statsPayload *pb.StatsPayload, origResource pcommon.Resource, outMetrics *pmetric.Metrics) error {
	for _, stat := range statsPayload.Stats {
		// Create resource metrics for each stat bucket
		rm := outMetrics.ResourceMetrics().AppendEmpty()

		// Copy original resource attributes
		origResource.CopyTo(rm.Resource())
		resourceAttrs := rm.Resource().Attributes()

		// Add DD-specific resource attributes if available
		if stat.Hostname != "" {
			resourceAttrs.PutStr("host.name", stat.Hostname)
		}
		if stat.Env != "" {
			resourceAttrs.PutStr("deployment.environment", stat.Env)
		}
		if stat.Service != "" {
			resourceAttrs.PutStr("service.name", stat.Service)
		}
		if stat.Version != "" {
			resourceAttrs.PutStr("service.version", stat.Version)
		}
		if stat.Lang != "" {
			resourceAttrs.PutStr("telemetry.sdk.language", stat.Lang)
		}
		if stat.TracerVersion != "" {
			resourceAttrs.PutStr("telemetry.sdk.version", stat.TracerVersion)
		}
		if stat.RuntimeID != "" {
			resourceAttrs.PutStr("process.runtime.id", stat.RuntimeID)
		}
		if stat.ContainerID != "" {
			resourceAttrs.PutStr("container.id", stat.ContainerID)
		}

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("signozddstatsprocessor")

		// Process each stats bucket
		for _, bucket := range stat.Stats {
			startNs := bucket.Start
			durationNs := bucket.Duration

			// Process each group in the bucket
			for _, group := range bucket.Stats {

				// Create metrics for this group
				p.createHitsMetric(sm, group, startNs, durationNs)
				p.createErrorsMetric(sm, group, startNs, durationNs)
				p.createDurationMetric(sm, group, startNs, durationNs)
			}
		}
	}

	return nil
}

func (p *ddStatsProcessor) createHitsMetric(sm pmetric.ScopeMetrics, group *pb.ClientGroupedStats, startNs uint64, durationNs uint64) {
	baseMetricName := fmt.Sprintf("trace.%s", group.Name)

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(baseMetricName + ".hits")
	metric.SetDescription("Number of hits (requests) for a given service and resource")
	metric.SetUnit("1")

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(startNs))
	dp.SetTimestamp(pcommon.Timestamp(startNs + durationNs))
	dp.SetIntValue(int64(group.Hits))

	p.addGroupAttributes(dp.Attributes(), group)

	// 2. Hits by HTTP status metric
	if group.HTTPStatusCode != 0 {
		hitsByStatusMetric := sm.Metrics().AppendEmpty()
		hitsByStatusMetric.SetName(baseMetricName + ".hits.by_http_status")
		hitsByStatusMetric.SetDescription("Number of hits (requests) for a given service and resource by HTTP status")
		hitsByStatusMetric.SetUnit("1")

		hitsByStatusSum := hitsByStatusMetric.SetEmptySum()
		hitsByStatusSum.SetIsMonotonic(true)
		hitsByStatusSum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

		hitsByStatusDp := hitsByStatusSum.DataPoints().AppendEmpty()
		hitsByStatusDp.SetStartTimestamp(pcommon.Timestamp(startNs))
		hitsByStatusDp.SetTimestamp(pcommon.Timestamp(startNs + durationNs))
		hitsByStatusDp.SetIntValue(int64(group.Hits))

		statusClass := fmt.Sprintf("%dxx", group.HTTPStatusCode/100)
		hitsByStatusDp.Attributes().PutStr("http.status_class", statusClass)

		p.addGroupAttributes(hitsByStatusDp.Attributes(), group)
	}
}

func (p *ddStatsProcessor) createErrorsMetric(sm pmetric.ScopeMetrics, group *pb.ClientGroupedStats, startNs uint64, durationNs uint64) {
	baseMetricName := fmt.Sprintf("trace.%s", group.Name)

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(baseMetricName + ".errors")
	metric.SetDescription("Number of errors for a given service and resource")
	metric.SetUnit("1")

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(startNs))
	dp.SetTimestamp(pcommon.Timestamp(startNs + durationNs))
	dp.SetIntValue(int64(group.Errors))

	p.addGroupAttributes(dp.Attributes(), group)

	if group.Errors > 0 && group.HTTPStatusCode != 0 {
		errorsByStatusMetric := sm.Metrics().AppendEmpty()
		errorsByStatusMetric.SetName(baseMetricName + ".errors.by_http_status")
		errorsByStatusMetric.SetDescription("Number of errors for a given service and resource by HTTP status")
		errorsByStatusMetric.SetUnit("1")

		errorsByStatusSum := errorsByStatusMetric.SetEmptySum()
		errorsByStatusSum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		errorsByStatusSum.SetIsMonotonic(true)

		errorsByStatusDp := errorsByStatusSum.DataPoints().AppendEmpty()
		errorsByStatusDp.SetStartTimestamp(pcommon.Timestamp(startNs))
		errorsByStatusDp.SetTimestamp(pcommon.Timestamp(startNs + durationNs))
		errorsByStatusDp.SetIntValue(int64(group.Errors))

		statusClass := fmt.Sprintf("%dxx", group.HTTPStatusCode/100)
		errorsByStatusDp.Attributes().PutStr("http.status_class", statusClass)

		p.addGroupAttributes(errorsByStatusDp.Attributes(), group)

	}

}

func (p *ddStatsProcessor) createDurationMetric(sm pmetric.ScopeMetrics, group *pb.ClientGroupedStats, startNs uint64, durationNs uint64) {
	baseMetricName := fmt.Sprintf("trace.%s", group.Name)

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(baseMetricName)
	metric.SetDescription("Total duration (nanoseconds) for a given service and resource")
	metric.SetUnit("ns")

	durationHistogram := metric.SetEmptyHistogram()
	durationHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := durationHistogram.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(startNs))
	dp.SetTimestamp(pcommon.Timestamp(startNs + durationNs))
	dp.SetCount(group.Hits)
	dp.SetSum(float64(group.Duration))

	// Calculate average for min/max approximation
	if group.Hits > 0 {
		avgDuration := float64(group.Duration) / float64(group.Hits)
		dp.SetMin(avgDuration)
		dp.SetMax(avgDuration)
	}

	p.addGroupAttributes(dp.Attributes(), group)
}

func (p *ddStatsProcessor) addGroupAttributes(attrs pcommon.Map, group *pb.ClientGroupedStats) {
	if group.Service != "" {
		attrs.PutStr(attrService, group.Service)
	}
	if group.Resource != "" {
		attrs.PutStr(attrResource, group.Resource)
	}
	if group.HTTPStatusCode != 0 {
		attrs.PutInt(attrStatusCode, int64(group.HTTPStatusCode))
	}
	if group.Type != "" {
		attrs.PutStr(attrType, group.Type)
	}
	if group.Synthetics {
		attrs.PutBool("synthetics", true)
	}

	// Add span kind if available
	if group.SpanKind != "" {
		attrs.PutStr(attrSpanKind, group.SpanKind)
	}
}
