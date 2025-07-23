package signozmeterconnector

import (
	"context"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozmeterconnector/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/ptracesgen"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

var (
	defaultDeploymentEnvironment = "default_deployment_environment"
)

func TestTracesToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "custom_dimensions",
			cfg: &Config{
				Dimensions: []Dimension{
					{
						Name: "resource.0",
					},
				},
				MetricsFlushInterval: time.Millisecond,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateTracesToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testSpans := ptracesgen.Generate(ptracesgen.WithSpanCount(10), ptracesgen.WithResourceAttributeStringValue("unknown_service"))
			assert.NoError(t, conn.ConsumeTraces(context.Background(), testSpans))

			// Wait for the metrics to be flushed based on the MetricsFlushInterval
			time.Sleep(tc.cfg.MetricsFlushInterval * 2)
			// only 2 metrics for span size and span count will be exported
			assert.Equal(t, 2, sink.DataPointCount())

		})
	}
}

func TestMetricsToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "custom_dimensions",
			cfg: &Config{
				Dimensions: []Dimension{
					{
						Name: "resource.attr_0",
					},
				},
				MetricsFlushInterval: time.Millisecond,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateMetricsToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testMetrics := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
				GaugeMetricsCount:   10,
				GaugeDataPointCount: 10,
			},
			), pmetricsgen.WithResourceAttributeStringValue("unknown_service"))
			assert.NoError(t, conn.ConsumeMetrics(context.Background(), testMetrics))

			// Wait for the metrics to be flushed based on the MetricsFlushInterval
			time.Sleep(tc.cfg.MetricsFlushInterval * 2)
			// only 2 metrics for span size and span count will be exported
			assert.Equal(t, 2, sink.DataPointCount())

		})
	}
}

func TestLogsToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "custom_dimensions",
			cfg: &Config{
				Dimensions: []Dimension{
					{
						Name: "resource.0",
					},
				},
				MetricsFlushInterval: time.Millisecond,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateLogsToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testSpans := plogsgen.Generate()
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testSpans))

			// Wait for the metrics to be flushed based on the MetricsFlushInterval
			time.Sleep(tc.cfg.MetricsFlushInterval * 2)
			// only 2 metrics for span size and span count will be exported
			assert.Equal(t, 2, sink.DataPointCount())

		})
	}
}

func TestBuildKeyFromResourceBasedDimensions(t *testing.T) {
	testCases := []struct {
		Name               string
		Dimensions         []Dimension
		ResourceAttributes map[string]any
		expected           string
	}{
		{
			Name:               "nil_dimensions",
			Dimensions:         []Dimension{{Name: "service.name"}, {Name: "deployment.environment"}},
			ResourceAttributes: map[string]any{"k8s.deployment.name": "my_deployment"},
			expected:           "\x00\x00",
		},
		{
			Name:               "missing_dimensions_with_defaults",
			Dimensions:         []Dimension{{Name: "service.name", Default: &defaultServiceName}, {Name: "deployment.environment", Default: &defaultDeploymentEnvironment}},
			ResourceAttributes: map[string]any{"k8s.deployment.name": "my_deployment"},
			expected:           "\x00unknown_service\x00default_deployment_environment",
		},
		{
			Name:               "partial_missing_dimensions_without_defaults",
			Dimensions:         []Dimension{{Name: "service.name"}, {Name: "deployment.environment"}},
			ResourceAttributes: map[string]any{"deployment.environment": "my_dev_deployment"},
			expected:           "\x00\x00my_dev_deployment",
		},
		{
			Name:               "partial_missing_dimensions_with_defaults",
			Dimensions:         []Dimension{{Name: "service.name", Default: &defaultServiceName}, {Name: "deployment.environment", Default: &defaultDeploymentEnvironment}},
			ResourceAttributes: map[string]any{"deployment.environment": "my_dev_deployment"},
			expected:           "\x00unknown_service\x00my_dev_deployment",
		},
	}

	for _, tc := range testCases {
		connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: tc.Dimensions, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
		require.NoError(t, err)

		resourceAttrMap := pcommon.NewMap()
		err = resourceAttrMap.FromRaw(tc.ResourceAttributes)
		require.NoError(t, err)

		metricKey := connector.buildKeyFromResourceBasedOnDimensions(resourceAttrMap)
		assert.Equal(t, resourceMetricKey(tc.expected), metricKey)
	}
}

func TestBuildDimensionsMapFromResourceAttributes(t *testing.T) {
	testCases := []struct {
		Name               string
		Dimensions         []Dimension
		ResourceAttributes map[string]any
		expected           map[string]any
	}{
		{
			Name:               "nil_dimensions",
			Dimensions:         []Dimension{{Name: "service.name"}, {Name: "deployment.environment"}},
			ResourceAttributes: map[string]any{"k8s.deployment.name": "my_deployment"},
			expected:           map[string]any{},
		},
		{
			Name:               "missing_dimensions_with_defaults",
			Dimensions:         []Dimension{{Name: "service.name", Default: &defaultServiceName}, {Name: "deployment.environment", Default: &defaultDeploymentEnvironment}},
			ResourceAttributes: map[string]any{"k8s.deployment.name": "my_deployment"},
			expected:           map[string]any{"service.name": defaultServiceName, "deployment.environment": defaultDeploymentEnvironment},
		},
		{
			Name:               "partial_missing_dimensions_without_defaults",
			Dimensions:         []Dimension{{Name: "service.name"}, {Name: "deployment.environment"}},
			ResourceAttributes: map[string]any{"deployment.environment": "my_dev_deployment"},
			expected:           map[string]any{"deployment.environment": "my_dev_deployment"},
		},
		{
			Name:               "partial_missing_dimensions_with_defaults",
			Dimensions:         []Dimension{{Name: "service.name", Default: &defaultServiceName}, {Name: "deployment.environment", Default: &defaultDeploymentEnvironment}},
			ResourceAttributes: map[string]any{"deployment.environment": "my_dev_deployment"},
			expected:           map[string]any{"service.name": defaultServiceName, "deployment.environment": "my_dev_deployment"},
		},
	}

	for _, tc := range testCases {
		connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: tc.Dimensions, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
		require.NoError(t, err)

		expectedDimensionsMap := pcommon.NewMap()
		err = expectedDimensionsMap.FromRaw(tc.expected)
		require.NoError(t, err)

		resourceAttrMap := pcommon.NewMap()
		err = resourceAttrMap.FromRaw(tc.ResourceAttributes)
		require.NoError(t, err)

		dimensionsMap := connector.buildDimensionsMapFromResourceAttributes(resourceAttrMap)
		equal := dimensionsMap.Equal(expectedDimensionsMap)
		assert.True(t, equal)
	}
}

func TestAggregateMeterMetricsFromTraces(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	testSpans := ptracesgen.Generate(ptracesgen.WithSpanCount(10), ptracesgen.WithResourceAttributeStringValue("unknown_service"))
	connector.aggregateMeterMetricsFromTraces(testSpans)

	expectedData := map[resourceMetricKey]meterMetrics{
		"\x00unknown_service": {
			SpanCount: 10,
			SpanSize:  4150,
		},
	}

	expectedDimensionsData := map[resourceMetricKey]pcommon.Map{
		"\x00unknown_service": func() pcommon.Map {
			m := pcommon.NewMap()
			m.PutStr("resource.0", "unknown_service")
			return m
		}(),
	}

	assert.Equal(t, expectedData, connector.data)
	assert.Equal(t, expectedDimensionsData, connector.dimensionsData)
}

func TestAggregateMeterMetricsFromMetrics(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.attr_0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	testMetrics := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		GaugeMetricsCount:   10,
		GaugeDataPointCount: 10,
	}), pmetricsgen.WithResourceAttributeStringValue("unknown_service"))
	connector.aggregateMeterMetricsFromMetrics(testMetrics)

	expectedData := map[resourceMetricKey]meterMetrics{
		"\x00unknown_service0": {
			MetricDataPointCount: 100,
			MetricDataPointSize:  0,
		},
	}

	expectedDimensionsData := map[resourceMetricKey]pcommon.Map{
		"\x00unknown_service0": func() pcommon.Map {
			m := pcommon.NewMap()
			m.PutStr("resource.attr_0", "unknown_service0")
			return m
		}(),
	}

	assert.Equal(t, expectedData, connector.data)
	assert.Equal(t, expectedDimensionsData, connector.dimensionsData)
}

func TestAggregateMeterMetricsFromLogs(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	testLogs := plogsgen.Generate(plogsgen.WithLogRecordCount(10), plogsgen.WithResourceAttributeStringValue("unknown_service"))
	connector.aggregateMeterMetricsFromLogs(testLogs)

	expectedData := map[resourceMetricKey]meterMetrics{
		"\x00unknown_service": {
			LogCount: 10,
			LogSize:  590,
		},
	}

	expectedDimensionsData := map[resourceMetricKey]pcommon.Map{
		"\x00unknown_service": func() pcommon.Map {
			m := pcommon.NewMap()
			m.PutStr("resource.0", "unknown_service")
			return m
		}(),
	}

	assert.Equal(t, expectedData, connector.data)
	assert.Equal(t, expectedDimensionsData, connector.dimensionsData)
}

func TestCollectTraceMeterMetrics(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	scopeMetrics := pmetric.NewScopeMetrics()
	resourceMetricKey := resourceMetricKey("\x00unknown_service")
	meterMetrics := meterMetrics{
		SpanCount: 10,
		SpanSize:  590,
	}
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	connector.collectTraceMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)

	metrics := scopeMetrics.Metrics()
	require.Equal(t, 2, metrics.Len())

	metric := metrics.At(0)
	assert.Equal(t, metricNameSpansCount, metric.Name())
	assert.Equal(t, int64(10), metric.Sum().DataPoints().At(0).IntValue())

	metric = metrics.At(1)
	assert.Equal(t, metricNameSpansSize, metric.Name())
	assert.Equal(t, int64(590), metric.Sum().DataPoints().At(0).IntValue())
}

func TestCollectMetricMeterMetrics(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	scopeMetrics := pmetric.NewScopeMetrics()
	resourceMetricKey := resourceMetricKey("\x00unknown_service")
	meterMetrics := meterMetrics{
		MetricDataPointCount: 100,
		MetricDataPointSize:  0,
	}
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	connector.collectMetricMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)

	metrics := scopeMetrics.Metrics()
	require.Equal(t, 2, metrics.Len())

	metric := metrics.At(0)
	assert.Equal(t, metricNameMetricsDataPointsCount, metric.Name())
	assert.Equal(t, int64(100), metric.Sum().DataPoints().At(0).IntValue())

	metric = metrics.At(1)
	assert.Equal(t, metricNameMetricsDataPointsSize, metric.Name())
	assert.Equal(t, int64(0), metric.Sum().DataPoints().At(0).IntValue())
}

func TestCollectLogMeterMetrics(t *testing.T) {
	connector, err := newConnector(zaptest.NewLogger(t), &Config{Dimensions: []Dimension{{Name: "resource.0"}}, MetricsFlushInterval: time.Second}, clockwork.NewFakeClock())
	require.NoError(t, err)

	scopeMetrics := pmetric.NewScopeMetrics()
	resourceMetricKey := resourceMetricKey("\x00unknown_service")
	meterMetrics := meterMetrics{
		LogCount: 10,
		LogSize:  590,
	}
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	connector.collectLogMeterMetrics(scopeMetrics, resourceMetricKey, meterMetrics, timestamp)

	metrics := scopeMetrics.Metrics()
	require.Equal(t, 2, metrics.Len())

	metric := metrics.At(0)
	assert.Equal(t, metricNameLogsCount, metric.Name())
	assert.Equal(t, int64(10), metric.Sum().DataPoints().At(0).IntValue())

	metric = metrics.At(1)
	assert.Equal(t, metricNameLogsSize, metric.Name())
	assert.Equal(t, int64(590), metric.Sum().DataPoints().At(0).IntValue())
}
