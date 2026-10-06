package signozlogsnormalizeprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	cases := []struct {
		id       component.ID
		expected Config
	}{
		{id: component.NewID(factory.Type()), expected: Config{MessageFields: []string{"log", "msg"}}},
		{id: component.NewIDWithName(factory.Type(), "dual"), expected: Config{JSONBodyDualIngestion: true, MessageFields: []string{"log", "msg"}}},
		{id: component.NewIDWithName(factory.Type(), "fields"), expected: Config{MessageFields: []string{"text"}}},
	}
	for _, tc := range cases {
		t.Run(tc.id.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tc.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			require.Equal(t, tc.expected, *cfg.(*Config))
		})
	}

	t.Run("unknown keys are rejected", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(factory.Type(), "unknown").String())
		require.NoError(t, err)
		require.Error(t, sub.Unmarshal(cfg))
	})
}
