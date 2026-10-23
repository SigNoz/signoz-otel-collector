package signozfilterprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	testCases := []struct {
		name      string
		id        component.ID
		expected  Config
		expectErr bool
	}{
		{name: "Default", id: component.NewID(factory.Type()), expected: Config{MaxAllowedDataAgeDays: 15}},
		{name: "CustomMaxAllowedDataAgeDays", id: component.NewIDWithName(factory.Type(), "custom"), expected: Config{MaxAllowedDataAgeDays: 3}},
		{name: "ZeroMaxAllowedDataAgeDays_Rejected", id: component.NewIDWithName(factory.Type(), "zero_max_allowed_data_age_days"), expectErr: true},
		{name: "UnknownKey_Rejected", id: component.NewIDWithName(factory.Type(), "unknown"), expectErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)
			if testCase.expectErr {
				if err := sub.Unmarshal(cfg); err == nil {
					assert.Error(t, xconfmap.Validate(cfg))
				}
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, testCase.expected, *cfg.(*Config))
		})
	}
}
