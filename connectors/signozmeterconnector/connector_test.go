package signozmeterconnector

import (
	"context"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozmeterconnector/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/ptracesgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
