package signozllmpricingprocessor

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

		assert.Equal(t, "gen_ai.request.model", cfg.Attrs.Model)
		assert.Equal(t, "gen_ai.usage.input_tokens", cfg.Attrs.In)
		assert.Equal(t, "gen_ai.usage.output_tokens", cfg.Attrs.Out)
		assert.Equal(t, "gen_ai.usage.input_token_details.cached", cfg.Attrs.CacheRead)
		assert.Equal(t, "gen_ai.usage.input_token_details.cache_creation", cfg.Attrs.CacheWrite)

		assert.Equal(t, UnitPerMillionTokens, cfg.DefaultPricing.Unit)
		require.Len(t, cfg.DefaultPricing.Rules, 2)

		gpt := cfg.DefaultPricing.Rules[0]
		assert.Equal(t, "gpt-4o", gpt.Name)
		assert.Equal(t, []string{"gpt-4o*"}, gpt.Pattern)
		assert.Equal(t, CacheModeSubtract, gpt.Cache.Mode)
		assert.Equal(t, 5.0, gpt.In)
		assert.Equal(t, 15.0, gpt.Out)
		assert.Equal(t, 2.5, gpt.Cache.Read)
		assert.Equal(t, 0.0, gpt.Cache.Write)

		claude := cfg.DefaultPricing.Rules[1]
		assert.Equal(t, "claude", claude.Name)
		assert.Equal(t, []string{"claude-*"}, claude.Pattern)
		assert.Equal(t, CacheModeAdditive, claude.Cache.Mode)
		assert.Equal(t, 3.0, claude.In)
		assert.Equal(t, 15.0, claude.Out)
		assert.Equal(t, 0.30, claude.Cache.Read)
		assert.Equal(t, 3.75, claude.Cache.Write)

		assert.Equal(t, "_signoz.gen_ai.cost_input", cfg.OutputAttrs.In)
		assert.Equal(t, "_signoz.gen_ai.cost_output", cfg.OutputAttrs.Out)
		assert.Equal(t, "_signoz.gen_ai.cost_cache_read", cfg.OutputAttrs.CacheRead)
		assert.Equal(t, "_signoz.gen_ai.cost_cache_write", cfg.OutputAttrs.CacheWrite)
		assert.Equal(t, "_signoz.gen_ai.total_cost", cfg.OutputAttrs.Total)
	})

	t.Run("minimal", func(t *testing.T) {
		cfg, err := loadConfig(t, component.NewIDWithName(processorType, "minimal"))
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())

		// Optional output attrs are empty — only total is required.
		assert.Empty(t, cfg.OutputAttrs.In)
		assert.Empty(t, cfg.OutputAttrs.CacheRead)
		assert.Equal(t, "_signoz.gen_ai.total_cost", cfg.OutputAttrs.Total)
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
			name:        "no_model_attr",
			id:          component.NewIDWithName(processorType, "no_model_attr"),
			errContains: "attrs.model must not be empty",
		},
		{
			name:        "bad_unit",
			id:          component.NewIDWithName(processorType, "bad_unit"),
			errContains: "is not supported",
		},
		{
			name:        "no_pattern",
			id:          component.NewIDWithName(processorType, "no_pattern"),
			errContains: "pattern must not be empty",
		},
		{
			name:        "bad_cache_mode",
			id:          component.NewIDWithName(processorType, "bad_cache_mode"),
			errContains: "cache.mode must be",
		},
		{
			name:        "no_total_output",
			id:          component.NewIDWithName(processorType, "no_total_output"),
			errContains: "output_attrs.total must not be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(t, tc.id)
			require.NoError(t, err)

			err = cfg.Validate()
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.errContains)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	_, ok := cfg.(*Config)
	require.True(t, ok)
}
