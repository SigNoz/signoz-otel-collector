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
		name      string
		id        component.ID
		expected  Config
		expectErr bool
	}{
		{name: "Default_BodyEnabled", id: component.NewID(factory.Type()), expected: Config{Body: BodyConfig{Enabled: true, MessageFields: []string{"log", "msg"}}}},
		{name: "BodyDisabled", id: component.NewIDWithName(factory.Type(), "body_disabled"), expected: Config{Body: BodyConfig{MessageFields: []string{"log", "msg"}}}},
		{name: "BodyDualIngestion", id: component.NewIDWithName(factory.Type(), "dual"), expected: Config{Body: BodyConfig{Enabled: true, JSONBodyDualIngestion: true, MessageFields: []string{"log", "msg"}}}},
		{name: "BodyCustomMessageFields", id: component.NewIDWithName(factory.Type(), "fields"), expected: Config{Body: BodyConfig{Enabled: true, MessageFields: []string{"text"}}}},
		{name: "UnknownKey_Rejected", id: component.NewIDWithName(factory.Type(), "unknown"), expectErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)
			if testCase.expectErr {
				assert.Error(t, sub.Unmarshal(cfg))
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))
			assert.Equal(t, testCase.expected, *cfg.(*Config))
		})
	}
}
