package clickhousesystemtablesreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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
			require.Error(xconfmap.Validate(cfg))
		} else {
			require.NoError(xconfmap.Validate(cfg))

		}
	}

}
