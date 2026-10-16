package signozlogsnormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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
		{name: "Default", id: component.NewID(factory.Type()), expected: withDefaults(func(*Config) {})},
		{name: "BodyDisabled", id: component.NewIDWithName(factory.Type(), "body_disabled"), expected: withDefaults(func(c *Config) { c.Body.Enabled = false })},
		{name: "BodyCustomFields", id: component.NewIDWithName(factory.Type(), "body_fields"), expected: withDefaults(func(c *Config) { c.Body.Fields = []string{"text"} })},
		{
			name: "FieldsCustomNames_OthersDefault",
			id:   component.NewIDWithName(factory.Type(), "fields"),
			expected: withDefaults(func(c *Config) {
				c.SeverityText.Fields = []string{"sev"}
				c.ScopeName.Fields = []string{"logger", "scope.name"}
			}),
		},
		{name: "FieldCopy_OthersMove", id: component.NewIDWithName(factory.Type(), "fields_copy"), expected: withDefaults(func(c *Config) { c.TraceID.Move = false })},
		{name: "FieldsEmptyList_KeptEmpty", id: component.NewIDWithName(factory.Type(), "fields_empty"), expected: withDefaults(func(c *Config) { c.SeverityText.Fields = []string{} })},
		{name: "OpAMPPositionBefore", id: component.NewIDWithName(factory.Type(), "opamp_position_before"), expected: withDefaults(func(c *Config) { c.OpAMP.Position = PositionBefore })},
		{name: "InvalidOpAMPPosition_Rejected", id: component.NewIDWithName(factory.Type(), "bad_opamp_position"), expectErr: true},
		{name: "UnknownBodyKey_Rejected", id: component.NewIDWithName(factory.Type(), "unknown"), expectErr: true},
		{name: "UnknownFieldKey_Rejected", id: component.NewIDWithName(factory.Type(), "field_unknown"), expectErr: true},
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
