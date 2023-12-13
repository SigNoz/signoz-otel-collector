// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signozspanmetricsprocessor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tilinna/clock"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
)

const (
	// The value of "type" key in configuration.
	typeStr = "signozspanmetrics"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var (
	spanMetricsProcessingTimeMillis = stats.Float64("signoz_spanmetrics_processor_processing_time_millis", "Time spent processing a span", stats.UnitMilliseconds)
)

// NewFactory creates a factory for the spanmetrics processor.
func NewFactory() processor.Factory {
	processingTimeDistribution := view.Distribution(100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1100, 1200, 1300, 1400, 1500, 1750, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 256000, 512000)
	processingTimeView := &view.View{
		Name:        spanMetricsProcessingTimeMillis.Name(),
		Measure:     spanMetricsProcessingTimeMillis,
		Description: spanMetricsProcessingTimeMillis.Description(),
		TagKeys:     []tag.Key{tag.MustNewKey("component")},
		Aggregation: processingTimeDistribution,
	}

	view.Register(processingTimeView)
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality: "AGGREGATION_TEMPORALITY_CUMULATIVE",
		DimensionsCacheSize:    defaultDimensionsCacheSize,
		skipSanitizeLabel:      dropSanitizationFeatureGate.IsEnabled(),
		MetricsFlushInterval:   60 * time.Second,
	}
}

func createTracesProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	var instanceID string
	serviceInstanceId, ok := params.Resource.Attributes().Get(semconv.AttributeServiceInstanceID)
	if ok {
		instanceID = serviceInstanceId.AsString()
	} else {
		instanceUUID, _ := uuid.NewRandom()
		instanceID = instanceUUID.String()
	}
	p, err := newProcessor(params.Logger, instanceID, cfg, metricsTicker(ctx, cfg))
	if err != nil {
		return nil, err
	}
	p.tracesConsumer = nextConsumer
	return p, nil
}

func metricsTicker(ctx context.Context, cfg component.Config) *clock.Ticker {
	return clock.FromContext(ctx).NewTicker(cfg.(*Config).MetricsFlushInterval)
}
