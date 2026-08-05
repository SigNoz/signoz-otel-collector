package clickhousesystemtablesreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		name             string
		testConfFileName string
		expectedErr      bool
	}{
		{
			name:             "valid",
			testConfFileName: "config.yaml",
			expectedErr:      false,
		}, {
			name:             "no dsn specified",
			testConfFileName: "badConfigNoDsn.yaml",
			expectedErr:      true,
		}, {
			name:             "no min scrape delay specified",
			testConfFileName: "badConfigNoScrapeDelay.yaml",
			expectedErr:      true,
		},
	}

	for _, tt := range tests {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig()

		cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.testConfFileName))
		require.Nil(err)

		sub, err := cm.Sub("clickhousesystemtablesreceiver")
		require.Nil(err)

		require.NoError(sub.Unmarshal(cfg))

		if tt.expectedErr {
			require.Error(confmap.Validate(cfg))
		} else {
			require.NoError(confmap.Validate(cfg))

		}
	}

}

func TestLoadMetricsConfig(t *testing.T) {
	require := require.New(t)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(err)
	sub, err := cm.Sub("clickhousesystemtablesreceiver")
	require.NoError(err)
	require.NoError(sub.Unmarshal(cfg))

	c := cfg.(*Config)

	m := c.SystemTablesMetrics
	require.Equal(15*time.Second, m.CollectionInterval)

	require.True(m.Metrics.ClickhouseViewRefreshLastSuccessAge.Enabled)
	require.True(m.Metrics.ClickhouseViewRefreshException.Enabled)
}
