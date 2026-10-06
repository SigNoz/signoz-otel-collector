package signozlogsnormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	testCases := []struct {
		id       component.ID
		expected Config
	}{
		{id: component.NewID(factory.Type()), expected: Config{MessageFields: []string{"log", "msg"}}},
		{id: component.NewIDWithName(factory.Type(), "dual"), expected: Config{JSONBodyDualIngestion: true, MessageFields: []string{"log", "msg"}}},
		{id: component.NewIDWithName(factory.Type(), "fields"), expected: Config{MessageFields: []string{"text"}}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.id.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.Equal(t, testCase.expected, *cfg.(*Config))
		})
	}

	t.Run("UnknownKey_Rejected", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(factory.Type(), "unknown").String())
		require.NoError(t, err)
		assert.Error(t, sub.Unmarshal(cfg))
	})
}
