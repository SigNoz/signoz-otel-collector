package signozllmcostprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmcostprocessor"

import (
	"fmt"
	"path"
)

const (
	CacheModeSubtract = "subtract" // cache_read is already counted inside input_tokens (e.g. OpenAI)
	CacheModeAdditive = "additive" // cache_read is separate from input_tokens (e.g. Anthropic)

	UnitPerMillionTokens = "per_million_tokens"
)

// Config is the top-level processor configuration.
type Config struct {
	// Attrs maps logical token-field names to the actual span attribute keys.
	Attrs AttrMapping `mapstructure:"attrs"`

	// DefaultPricing holds the ordered pricing rules applied when no
	// model-specific override exists.
	DefaultPricing PricingConfig `mapstructure:"default_pricing"`

	// OutputAttrs maps logical cost-field names to the span attribute keys
	// where computed costs are written.
	OutputAttrs OutputMapping `mapstructure:"output_attrs"`
}

// AttrMapping declares which span attributes carry token counts.
type AttrMapping struct {
	Model      string `mapstructure:"model"`
	In         string `mapstructure:"in"`
	Out        string `mapstructure:"out"`
	CacheRead  string `mapstructure:"cache_read"`
	CacheWrite string `mapstructure:"cache_write"`
}

// PricingConfig groups the unit declaration and the ordered rule list.
type PricingConfig struct {
	// Unit must be "per_million_tokens".
	Unit string `mapstructure:"unit"`

	// Rules is an ordered list of pricing rules. The first rule whose Pattern
	// glob-matches the model name wins.
	Rules []PricingRule `mapstructure:"rules"`
}

// PricingRule associates a glob pattern with per-token prices and a cache mode.
type PricingRule struct {
	// Pattern is a glob matched against the model name attribute.
	// Standard glob syntax: * matches any sequence, ? matches one character.
	Pattern string `mapstructure:"pattern"`

	// CacheMode controls how cache tokens are factored into the cost:
	//   "subtract" — cache_read tokens are already counted inside input_tokens;
	//                billed_input = input_tokens - cache_read.
	//   "additive" — cache_read/write are separate from input_tokens;
	//                all four buckets are billed independently.
	CacheMode string `mapstructure:"cache_mode"`

	// Per-million-token prices (USD).
	In         float64 `mapstructure:"in"`
	Out        float64 `mapstructure:"out"`
	CacheRead  float64 `mapstructure:"cache_read"`
	CacheWrite float64 `mapstructure:"cache_write"`
}

// OutputMapping declares where computed cost values are written.
type OutputMapping struct {
	In         string `mapstructure:"in"`
	Out        string `mapstructure:"out"`
	CacheRead  string `mapstructure:"cache_read"`
	CacheWrite string `mapstructure:"cache_write"`
	Total      string `mapstructure:"total"`
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if c.Attrs.Model == "" {
		return fmt.Errorf("attrs.model must not be empty")
	}

	unit := c.DefaultPricing.Unit
	if unit == "" {
		unit = UnitPerMillionTokens
	}
	if unit != UnitPerMillionTokens {
		return fmt.Errorf("default_pricing.unit %q is not supported, must be %q", unit, UnitPerMillionTokens)
	}

	for i, r := range c.DefaultPricing.Rules {
		if r.Pattern == "" {
			return fmt.Errorf("default_pricing.rules[%d]: pattern must not be empty", i)
		}
		if _, err := path.Match(r.Pattern, ""); err != nil {
			return fmt.Errorf("default_pricing.rules[%d]: invalid glob pattern %q: %w", i, r.Pattern, err)
		}
		if r.CacheMode != CacheModeSubtract && r.CacheMode != CacheModeAdditive {
			return fmt.Errorf("default_pricing.rules[%d] (pattern=%q): cache_mode must be %q or %q, got %q",
				i, r.Pattern, CacheModeSubtract, CacheModeAdditive, r.CacheMode)
		}
		if r.In < 0 || r.Out < 0 || r.CacheRead < 0 || r.CacheWrite < 0 {
			return fmt.Errorf("default_pricing.rules[%d] (pattern=%q): prices must be non-negative", i, r.Pattern)
		}
	}

	if c.OutputAttrs.Total == "" {
		return fmt.Errorf("output_attrs.total must not be empty")
	}

	return nil
}
