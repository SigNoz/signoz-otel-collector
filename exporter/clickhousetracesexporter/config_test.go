package clickhousetracesexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousetracesexporter/internal/metadata"
)

// TestLoadConfig checks whether yaml configuration can be loaded correctly
func Test_loadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// From the default configurations -- checks if a correct exporter is instantiated
	e0 := cfg.Exporters[(component.NewID(metadata.Type))]
	defaultCfg := factory.CreateDefaultConfig()
	defaultCfg.(*Config).Datasource = "tcp://127.0.0.1:9000/?database=signoz_traces&username=admin&password=password"
	assert.Equal(t, e0, defaultCfg)

	// checks if the correct Config struct can be instantiated from testdata/config.yaml
	e1 := cfg.Exporters[component.NewIDWithName(metadata.Type, "2")]
	assert.Equal(t, e1,
		&Config{
			Datasource: "tcp://127.0.0.1:9000/?database=signoz_traces&username=admin&password=password",
			TimeoutConfig: exporterhelper.TimeoutConfig{
				Timeout: 5 * time.Second,
			},
			BackOffConfig: configretry.BackOffConfig{
				Enabled:             true,
				InitialInterval:     5 * time.Second,
				MaxInterval:         30 * time.Second,
				MaxElapsedTime:      300 * time.Second,
				RandomizationFactor: 0.7,
				Multiplier:          1.3,
			},
			QueueBatchConfig: exporterhelper.QueueBatchConfig{
				Enabled:      true,
				Sizer:        exporterhelper.RequestSizerTypeRequests,
				NumConsumers: 5,
				QueueSize:    100,
			},
			AttributesLimits: AttributesLimits{
				FetchKeysInterval: 10 * time.Minute,
				MaxDistinctValues: 25000,
			},
		})
}
