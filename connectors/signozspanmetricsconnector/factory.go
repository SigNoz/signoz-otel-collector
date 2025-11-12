package signozspanmetricsconnector

import (
	"context"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozspanmetricsconnector/internal/metadata"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	DefaultNamespace                           = "traces.span.metrics"
	legacyMetricNamesFeatureGateID             = "connector.spanmetrics.legacyMetricNames"
	includeCollectorInstanceIDFeatureGateID    = "connector.spanmetrics.includeCollectorInstanceID"
	useSecondAsDefaultMetricsUnitFeatureGateID = "connector.spanmetrics.useSecondAsDefaultMetricsUnit"
	excludeResourceMetricsFeatureGate          = "connector.spanmetrics.excludeResourceMetrics"
)

// NewFactory creates a factory for the spanmetrics connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality:         cumulative,
		DimensionsCacheSize:            defaultDimensionsCacheSize,
		skipSanitizeLabel:              dropSanitizationFeatureGate.IsEnabled(),
		EnableExpHistogram:             false,
		MetricsFlushInterval:           60 * time.Second,
		MaxServicesToTrack:             maxNumberOfServicesToTrack,
		MaxOperationsToTrackPerService: maxNumberOfOperationsToTrackPerService,
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	// TODO(nikhilmantri0902, srikanthccv): the processor uses serviceInstanceKey below, verify if using collector.instance.id is okay?
	instanceID, ok := params.Resource.Attributes().Get(collectorInstanceKey)
	// This never happens: the OpenTelemetry Collector automatically adds this attribute.
	// See: https://github.com/open-telemetry/opentelemetry-collector/blob/main/service/internal/resource/config.go#L31
	//
	// The fallback logic below exists solely for lifecycle tests in generated_component_test.go,
	// where the mocked telemetry setting does not include the service.instance.id attribute.
	if !ok {
		instanceUUID, _ := uuid.NewRandom()
		instanceID = pcommon.NewValueStr(instanceUUID.String())
	}

	c, err := newConnector(params.Logger, cfg, clockwork.FromContext(ctx), instanceID.AsString())
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}
