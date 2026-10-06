package signozlogsinferrerprocessor

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
		name     string
		id       component.ID
		expected Config
		wantErr  bool
	}{
		{
			name:     "Unspecified_NilFields",
			id:       component.NewID(factory.Type()),
			expected: Config{},
		},
		{
			name: "ConfiguredFields",
			id:   component.NewIDWithName(factory.Type(), "fields"),
			expected: Config{
				SeverityTextFields: []string{"sev"},
				ScopeNameFields:    []string{"logger", "scope.name"},
			},
		},
		{
			name:     "EmptyList_KeptEmpty",
			id:       component.NewIDWithName(factory.Type(), "empty"),
			expected: Config{SeverityTextFields: []string{}},
		},
		{
			name:    "UnknownKey_Rejected",
			id:      component.NewIDWithName(factory.Type(), "unknown"),
			wantErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)

			err = sub.Unmarshal(cfg)
			if testCase.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, *cfg.(*Config))
		})
	}
}
