package signozmeterconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozmeterconnector/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		name   string
		expect *Config
	}{
		{
			name: "default_config",
			expect: &Config{
				Dimensions: []Dimension{
					{
						Name: "service.name",
					},
					{
						Name: "deployment.environment",
					},
					{
						Name: "host.name",
					},
				},
				MetricsFlushInterval: time.Hour * 1,
			},
		},
		{
			name: "custom_dimensions",
			expect: &Config{
				Dimensions: []Dimension{
					{
						Name: "k8s.namespace.name",
					},
					{
						Name: "k8s.node.id",
					},
					{
						Name: "ingestion.key",
					},
				},
				MetricsFlushInterval: time.Hour * 1,
			},
		},
		{
			name: "custom_metrics_flush_interval",
			expect: &Config{
				Dimensions: []Dimension{
					{
						Name: "service.name",
					},
					{
						Name: "deployment.environment",
					},
					{
						Name: "host.name",
					},
				},
				MetricsFlushInterval: time.Minute * 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Equal(t, tc.expect, cfg)
		})
	}
}

func TestConfigErrors(t *testing.T) {
	testCases := []struct {
		name   string
		input  *Config
		expect string
	}{
		{
			name: "duplicate_dimensions",
			input: &Config{
				Dimensions: []Dimension{
					{
						Name: "host.name",
					},
					{
						Name: "host.name",
					},
				},
			},
			expect: "failed validating dimensions: duplicate dimension name host.name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			assert.ErrorContains(t, err, tc.expect)
		})
	}
}
