package metadataexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: exporterhelper.NewDefaultQueueConfig(),
				DSN:              "tcp://localhost:9000",
				MaxDistinctValues: MaxDistinctValuesConfig{
					Traces: LimitsConfig{
						MaxKeys:                 100,
						MaxStringLength:         128,
						MaxStringDistinctValues: 420,
						FetchInterval:           5 * time.Minute,
					},
					Logs: LimitsConfig{
						MaxKeys:                 100,
						MaxStringLength:         64,
						MaxStringDistinctValues: 512,
						FetchInterval:           10 * time.Minute,
					},
					Metrics: LimitsConfig{
						MaxKeys:                 100,
						MaxStringLength:         16,
						MaxStringDistinctValues: 1024,
						FetchInterval:           5 * time.Minute,
					},
				},
				Cache: CacheConfig{
					Provider: CacheProviderInMemory,
					InMemory: InMemoryCacheConfig{},
					Traces: CacheLimits{
						MaxResources:              DefaultMaxResources,
						MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
						MaxTotalCardinality:       DefaultMaxTotalCardinality,
					},
					Metrics: CacheLimits{
						MaxResources:              DefaultMaxResources,
						MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
						MaxTotalCardinality:       DefaultMaxTotalCardinality,
					},
					Logs: CacheLimits{
						MaxResources:              DefaultMaxResources,
						MaxCardinalityPerResource: DefaultMaxCardinalityPerResource,
						MaxTotalCardinality:       DefaultMaxTotalCardinality,
					},
				},
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
