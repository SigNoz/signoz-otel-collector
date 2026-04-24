package signozllmpricingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor"

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

// PricingRule associates one or more glob patterns with per-token prices and a
// cache mode. When multiple patterns are given, the rule matches if any one
// of them glob-matches the model name.
type PricingRule struct {
	// Name is a human-friendly identifier for the rule (e.g. the canonical
	// model name). Optional; used for logging and debugging only — it does
	// not affect matching.
	Name string `mapstructure:"name"`

	// Pattern is a list of globs matched against the model name attribute.
	// Standard glob syntax: * matches any sequence, ? matches one character.
	// At least one pattern is required.
	Pattern []string `mapstructure:"pattern"`

	// Cache controls how cache tokens are factored into the cost.
	Cache PricingRuleCache `mapstructure:"cache"`

	// Per-million-token prices (USD) for input and output tokens.
	In  float64 `mapstructure:"in"`
	Out float64 `mapstructure:"out"`
}

// PricingRuleCache holds the cache-accounting mode and per-million-token
// prices for cached tokens.
type PricingRuleCache struct {
	// Mode controls how cache tokens are factored into the cost:
	//   "subtract" — cache read tokens are already counted inside input_tokens;
	//                billed_input = input_tokens - cache_read.
	//   "additive" — cache read/write are separate from input_tokens;
	//                all four buckets are billed independently.
	Mode string `mapstructure:"mode"`

	// Per-million-token prices (USD) for cached reads/writes.
	Read  float64 `mapstructure:"read"`
	Write float64 `mapstructure:"write"`
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
		if len(r.Pattern) == 0 {
			return fmt.Errorf("default_pricing.rules[%d]: pattern must not be empty", i)
		}
		for j, p := range r.Pattern {
			if p == "" {
				return fmt.Errorf("default_pricing.rules[%d].pattern[%d]: pattern must not be empty", i, j)
			}
			if _, err := path.Match(p, ""); err != nil {
				return fmt.Errorf("default_pricing.rules[%d].pattern[%d]: invalid glob pattern %q: %w", i, j, p, err)
			}
		}
		if r.Cache.Mode != CacheModeSubtract && r.Cache.Mode != CacheModeAdditive {
			return fmt.Errorf("default_pricing.rules[%d] (pattern=%v): cache.mode must be %q or %q, got %q",
				i, r.Pattern, CacheModeSubtract, CacheModeAdditive, r.Cache.Mode)
		}
		if r.In < 0 || r.Out < 0 || r.Cache.Read < 0 || r.Cache.Write < 0 {
			return fmt.Errorf("default_pricing.rules[%d] (pattern=%v): prices must be non-negative", i, r.Pattern)
		}
	}

	if c.OutputAttrs.Total == "" {
		return fmt.Errorf("output_attrs.total must not be empty")
	}

	return nil
}
