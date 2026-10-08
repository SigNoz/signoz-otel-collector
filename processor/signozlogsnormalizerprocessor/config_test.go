package signozlogsnormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func withDefaults(mutate func(*Config)) Config {
	cfg := createDefaultConfig().(*Config)
	mutate(cfg)
	return *cfg
}

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
		{name: "Default_BodyAndFieldsEnabled", id: component.NewID(factory.Type()), expected: withDefaults(func(*Config) {})},
		{name: "BodyDisabled", id: component.NewIDWithName(factory.Type(), "body_disabled"), expected: withDefaults(func(c *Config) { c.Body.Enabled = false })},
		{name: "BodyCustomMessageFields", id: component.NewIDWithName(factory.Type(), "message_fields"), expected: withDefaults(func(c *Config) { c.Body.MessageFields = []string{"text"} })},
		{name: "FieldsDisabled", id: component.NewIDWithName(factory.Type(), "fields_disabled"), expected: withDefaults(func(c *Config) { c.Fields.Enabled = false })},
		{
			name: "FieldsCustomNames_OthersDefault",
			id:   component.NewIDWithName(factory.Type(), "fields"),
			expected: withDefaults(func(c *Config) {
				c.Fields.SeverityText = []string{"sev"}
				c.Fields.ScopeName = []string{"logger", "scope.name"}
			}),
		},
		{name: "FieldsEmptyList_KeptEmpty", id: component.NewIDWithName(factory.Type(), "fields_empty"), expected: withDefaults(func(c *Config) { c.Fields.SeverityText = []string{} })},
		{name: "UnknownBodyKey_Rejected", id: component.NewIDWithName(factory.Type(), "unknown"), expectErr: true},
		{name: "UnknownFieldsKey_Rejected", id: component.NewIDWithName(factory.Type(), "fields_unknown"), expectErr: true},
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
