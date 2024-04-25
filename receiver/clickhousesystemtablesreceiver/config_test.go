package clickhousesystemtablesreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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

		require.NoError(component.UnmarshalConfig(sub, cfg))

		if tt.expectedErr {
			require.Error(component.ValidateConfig(cfg))
		} else {
			require.NoError(component.ValidateConfig(cfg))

		}
	}

}
