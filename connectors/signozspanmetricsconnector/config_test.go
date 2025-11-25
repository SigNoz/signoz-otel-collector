package signozspanmetricsconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozspanmetricsconnector/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
)

func TestLoadConfig(t *testing.T) {
	defaultMethod := "GET"
	testcases := []struct {
		configFile                         string
		wantMetricsExporter                string
		wantLatencyHistogramBuckets        []time.Duration
		wantDimensions                     []Dimension
		wantDimensionsCacheSize            int
		wantAggregationTemporality         string
		wantMetricsFlushInterval           time.Duration
		wantMaxServicesToTrack             int
		wantMaxOperationsToTrackPerService int
		wantExcludePatterns                []ExcludePattern
		wantTimeBucketInterval             time.Duration
		wantEnableExpHistogram             bool
		wantSkipSpansOlderThan             time.Duration
	}{
		{
			configFile:                         "config-2-pipelines.yaml",
			wantMetricsExporter:                "prometheus",
			wantAggregationTemporality:         cumulative,
			wantDimensionsCacheSize:            500,
			wantMetricsFlushInterval:           30 * time.Second,
			wantMaxServicesToTrack:             256,
			wantMaxOperationsToTrackPerService: 2048,
		},
		{
			configFile:                         "config-3-pipelines.yaml",
			wantMetricsExporter:                "otlp/spanmetrics",
			wantAggregationTemporality:         cumulative,
			wantDimensionsCacheSize:            defaultDimensionsCacheSize,
			wantMetricsFlushInterval:           60 * time.Second,
			wantMaxServicesToTrack:             256,
			wantMaxOperationsToTrackPerService: 2048,
		},
		{
			configFile:          "config-full.yaml",
			wantMetricsExporter: "otlp/spanmetrics",
			wantLatencyHistogramBuckets: []time.Duration{
				100 * time.Microsecond,
				1 * time.Millisecond,
				2 * time.Millisecond,
				6 * time.Millisecond,
				10 * time.Millisecond,
				100 * time.Millisecond,
				250 * time.Millisecond,
			},
			wantDimensions: []Dimension{
				{"http.method", &defaultMethod},
				{"http.status_code", nil},
			},
			wantDimensionsCacheSize:            1500,
			wantAggregationTemporality:         delta,
			wantMetricsFlushInterval:           60 * time.Second,
			wantMaxServicesToTrack:             512,
			wantMaxOperationsToTrackPerService: 69420,
			wantExcludePatterns: []ExcludePattern{
				{Name: "operation", Pattern: "^/internal/"},
			},
			wantTimeBucketInterval: 30 * time.Second,
			wantEnableExpHistogram: true,
			wantSkipSpansOlderThan: 24 * time.Hour,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.configFile, func(t *testing.T) {
			// Prepare
			factories, err := otelcoltest.NopFactories()
			require.NoError(t, err)

			factories.Receivers[component.MustNewType("otlp")] = otlpreceiver.NewFactory()
			factories.Receivers[component.MustNewType("jaeger")] = jaegerreceiver.NewFactory()

			factories.Connectors[metadata.Type] = NewFactory()
			factories.Processors[component.MustNewType("batch")] = batchprocessor.NewFactory()

			factories.Exporters[component.MustNewType("otlp")] = otlpexporter.NewFactory()
			factories.Exporters[component.MustNewType("prometheus")] = prometheusexporter.NewFactory()

			// Test
			cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", tc.configFile), factories)

			// Verify
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t,
				&Config{
					LatencyHistogramBuckets:        tc.wantLatencyHistogramBuckets,
					Dimensions:                     tc.wantDimensions,
					DimensionsCacheSize:            tc.wantDimensionsCacheSize,
					ExcludePatterns:                tc.wantExcludePatterns,
					AggregationTemporality:         tc.wantAggregationTemporality,
					MetricsFlushInterval:           tc.wantMetricsFlushInterval,
					TimeBucketInterval:             tc.wantTimeBucketInterval,
					EnableExpHistogram:             tc.wantEnableExpHistogram,
					MaxServicesToTrack:             tc.wantMaxServicesToTrack,
					MaxOperationsToTrackPerService: tc.wantMaxOperationsToTrackPerService,
					SkipSpansOlderThan:             tc.wantSkipSpansOlderThan,
				},
				cfg.Connectors[component.NewID(metadata.Type)],
			)
		})
	}
}

func TestGetAggregationTemporality(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantResult pmetric.AggregationTemporality
	}{
		{
			name:       "delta temporality",
			config:     Config{AggregationTemporality: delta},
			wantResult: pmetric.AggregationTemporalityDelta,
		},
		{
			name:       "cumulative temporality",
			config:     Config{AggregationTemporality: cumulative},
			wantResult: pmetric.AggregationTemporalityCumulative,
		},
		{
			name:       "default temporality",
			config:     Config{},
			wantResult: pmetric.AggregationTemporalityCumulative,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.config.GetAggregationTemporality()
			assert.Equal(t, tc.wantResult, got)
		})
	}
}

func TestGetTimeBucketInterval(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantResult time.Duration
	}{
		{
			name:       "default time bucket",
			config:     Config{},
			wantResult: defaultTimeBucketInterval,
		},
		{
			name:       "custom time bucket",
			config:     Config{TimeBucketInterval: 30 * time.Second},
			wantResult: 30 * time.Second,
		},
		{
			name:       "another custom bucket",
			config:     Config{TimeBucketInterval: 2 * time.Minute},
			wantResult: 2 * time.Minute,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.config.GetTimeBucketInterval()
			assert.Equal(t, tc.wantResult, got)
		})
	}
}

func TestGetSkipSpansOlderThan(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantResult time.Duration
	}{
		{
			name:       "default skip window",
			config:     Config{},
			wantResult: defaultSkipSpansOlderThan,
		},
		{
			name:       "custom skip window",
			config:     Config{SkipSpansOlderThan: 12 * time.Hour},
			wantResult: 12 * time.Hour,
		},
		{
			name:       "zero value falls back to default",
			config:     Config{SkipSpansOlderThan: 0},
			wantResult: defaultSkipSpansOlderThan,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.config.GetSkipSpansOlderThan()
			assert.Equal(t, tc.wantResult, got)
		})
	}
}
