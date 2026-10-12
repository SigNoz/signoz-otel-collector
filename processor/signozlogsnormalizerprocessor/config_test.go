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
		{name: "Default_BodyEnabled", id: component.NewID(factory.Type()), expected: Config{OpAMP: OpAMPConfig{Position: PositionAfter}, Body: BodyConfig{Enabled: true, Fields: []string{"log", "msg"}}}},
		{name: "BodyDisabled", id: component.NewIDWithName(factory.Type(), "body_disabled"), expected: Config{OpAMP: OpAMPConfig{Position: PositionAfter}, Body: BodyConfig{Fields: []string{"log", "msg"}}}},
		{name: "BodyCustomFields", id: component.NewIDWithName(factory.Type(), "fields"), expected: Config{OpAMP: OpAMPConfig{Position: PositionAfter}, Body: BodyConfig{Enabled: true, Fields: []string{"text"}}}},
		{name: "OpAMPPositionBefore", id: component.NewIDWithName(factory.Type(), "opamp_position_before"), expected: Config{OpAMP: OpAMPConfig{Position: PositionBefore}, Body: BodyConfig{Enabled: true, Fields: []string{"log", "msg"}}}},
		{name: "InvalidOpAMPPosition_Rejected", id: component.NewIDWithName(factory.Type(), "bad_opamp_position"), expectErr: true},
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
