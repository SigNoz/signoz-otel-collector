package signozspanmappingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

var processorType = component.MustNewType(typeStr)

func loadConfig(t *testing.T, id component.ID) (*Config, error) {
	t.Helper()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)

	if err := sub.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg.(*Config), nil
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("full", func(t *testing.T) {
		cfg, err := loadConfig(t, component.NewID(processorType))
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())

		require.Len(t, cfg.Groups, 3)

		llm := cfg.Groups[0]
		assert.Equal(t, "llm", llm.ID)
		assert.Equal(t, []string{"*model*"}, llm.ExistsAny.Attributes)
		assert.Equal(t, []string{"service.name*"}, llm.ExistsAny.Resource)
		require.Len(t, llm.Attributes, 3)

		modelRule := llm.Attributes[0]
		assert.Equal(t, "gen_ai.request.model", modelRule.Target)
		assert.Equal(t, ContextResource, modelRule.Context)
		assert.Equal(t, []string{"gen_ai.llm.model", "llm.model", "resource.service.name"}, modelRule.Sources)
		assert.Empty(t, modelRule.Action) // default

		inputRule := llm.Attributes[2]
		assert.Equal(t, "gen_ai.request.input", inputRule.Target)
		assert.Equal(t, ActionMove, inputRule.Action)

		agent := cfg.Groups[1]
		assert.Equal(t, "agent", agent.ID)
		assert.Equal(t, []string{"agent.*"}, agent.ExistsAny.Attributes)
		assert.Empty(t, agent.ExistsAny.Resource)

		tool := cfg.Groups[2]
		assert.Equal(t, "tool", tool.ID)
	})

	t.Run("resource_only", func(t *testing.T) {
		cfg, err := loadConfig(t, component.NewIDWithName(processorType, "resource_only"))
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())

		require.Len(t, cfg.Groups, 1)
		g := cfg.Groups[0]
		assert.Empty(t, g.ExistsAny.Attributes)
		assert.Equal(t, []string{"service.*"}, g.ExistsAny.Resource)
	})

	t.Run("defaults", func(t *testing.T) {
		cfg, err := loadConfig(t, component.NewIDWithName(processorType, "defaults"))
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())

		rule := cfg.Groups[0].Attributes[0]
		assert.Empty(t, rule.Action)  // defaults to copy at runtime
		assert.Empty(t, rule.Context) // defaults to attributes at runtime
	})
}

func TestValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          component.ID
		errContains string
	}{
		{
			name:        "no_patterns",
			id:          component.NewIDWithName(processorType, "no_patterns"),
			errContains: "exists_any must have at least one",
		},
		{
			name:        "bad_glob",
			id:          component.NewIDWithName(processorType, "bad_glob"),
			errContains: "invalid glob pattern",
		},
		{
			name:        "empty_target",
			id:          component.NewIDWithName(processorType, "empty_target"),
			errContains: "target must not be empty",
		},
		{
			name:        "no_sources",
			id:          component.NewIDWithName(processorType, "no_sources"),
			errContains: "sources must not be empty",
		},
		{
			name:        "bad_context",
			id:          component.NewIDWithName(processorType, "bad_context"),
			errContains: "unknown context",
		},
		{
			name:        "bad_action",
			id:          component.NewIDWithName(processorType, "bad_action"),
			errContains: "unknown action",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(t, tc.id)
			require.NoError(t, err, "unmarshal must succeed; only Validate should fail")

			err = cfg.Validate()
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestValidateErrorMessages(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Groups: []Group{
			{
				ID:        "first",
				ExistsAny: ExistsAny{Attributes: []string{"*"}},
				Attributes: []AttributeRule{
					{Target: "ok", Sources: []string{"s"}},
				},
			},
			{
				ID:        "second",
				ExistsAny: ExistsAny{Attributes: []string{"*"}},
				Attributes: []AttributeRule{
					{Target: "", Sources: []string{"s"}}, // bad
				},
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, "group[1]")
	assert.ErrorContains(t, err, `id="second"`)
}

func TestIsResourceKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key      string
		wantBare string
		wantRes  bool
	}{
		{"resource.service.name", "service.name", true},
		{"resource.foo", "foo", true},
		{"resource.", "", true}, // edge: bare prefix only
		{"service.name", "service.name", false},
		{"gen_ai.model", "gen_ai.model", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			bare, isRes := isResourceKey(tc.key)
			assert.Equal(t, tc.wantBare, bare)
			assert.Equal(t, tc.wantRes, isRes)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	typed, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Empty(t, typed.Groups)
	// empty config is valid (no groups → nothing to validate)
	assert.NoError(t, typed.Validate())
}
