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
	"path/filepath"
	"testing"
	"time"

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

	"github.com/SigNoz/signoz-otel-collector/processor/signozspanmetricsprocessor/internal/metadata"
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
		},
	}
	for _, tc := range testcases {
		t.Run(tc.configFile, func(t *testing.T) {
			// Prepare
			factories, err := otelcoltest.NopFactories()
			require.NoError(t, err)

			factories.Receivers[component.MustNewType("otlp")] = otlpreceiver.NewFactory()
			factories.Receivers[component.MustNewType("jaeger")] = jaegerreceiver.NewFactory()

			factories.Processors[metadata.Type] = NewFactory()
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
					MetricsExporter:                tc.wantMetricsExporter,
					LatencyHistogramBuckets:        tc.wantLatencyHistogramBuckets,
					Dimensions:                     tc.wantDimensions,
					DimensionsCacheSize:            tc.wantDimensionsCacheSize,
					AggregationTemporality:         tc.wantAggregationTemporality,
					MetricsFlushInterval:           tc.wantMetricsFlushInterval,
					MaxServicesToTrack:             tc.wantMaxServicesToTrack,
					MaxOperationsToTrackPerService: tc.wantMaxOperationsToTrackPerService,
				},
				cfg.Processors[component.NewID(metadata.Type)],
			)
		})
	}
}

func TestGetAggregationTemporality(t *testing.T) {
	cfg := &Config{AggregationTemporality: delta}
	assert.Equal(t, pmetric.AggregationTemporalityDelta, cfg.GetAggregationTemporality())

	cfg = &Config{AggregationTemporality: cumulative}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())

	cfg = &Config{}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())
}

func TestGetTimeBucketInterval(t *testing.T) {
	cfg := &Config{}
	// Should return default when not set
	assert.Equal(t, defaultTimeBucketInterval, cfg.GetTimeBucketInterval())

	// Should return configured value when set
	cfg.TimeBucketInterval = 30 * time.Second
	assert.Equal(t, 30*time.Second, cfg.GetTimeBucketInterval())

	// Should return configured value when set to non-zero
	cfg.TimeBucketInterval = 2 * time.Minute
	assert.Equal(t, 2*time.Minute, cfg.GetTimeBucketInterval())
}
