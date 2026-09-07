package signozllmpricingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const ruleNameAttr = "_signoz.gen_ai.pricing_rule"

// cfgWithRuleName returns testCfg with the rule-name output attribute enabled.
func cfgWithRuleName() *Config {
	cfg := *testCfg
	cfg.OutputAttrs = testCfg.OutputAttrs
	cfg.OutputAttrs.RuleName = ruleNameAttr
	return &cfg
}

func getStr(t *testing.T, m pcommon.Map, key string) string {
	t.Helper()
	v, ok := m.Get(key)
	require.True(t, ok, "expected attribute %q to be present", key)
	return v.Str()
}

// The matched rule's name must land on the configured key in subtract mode.
func TestRuleName_SubtractMode(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":           "gpt-4o-mini",
		"gen_ai.usage.input_tokens":      int64(1000),
		"gen_ai.usage.output_tokens":     int64(500),
		"gen_ai.usage.cache_read_tokens": int64(200),
	})

	_, err := newProcessor(cfgWithRuleName()).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.Equal(t, "gpt-4o", getStr(t, a, ruleNameAttr))
	// Costs must still be computed exactly as before.
	assert.InDelta(t, 0.012, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

// ...and in additive mode, which takes a different branch through compute.
func TestRuleName_AdditiveMode(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":            "claude-3-5-sonnet",
		"gen_ai.usage.input_tokens":       int64(1000),
		"gen_ai.usage.output_tokens":      int64(500),
		"gen_ai.usage.cache_read_tokens":  int64(200),
		"gen_ai.usage.cache_write_tokens": int64(100),
	})

	_, err := newProcessor(cfgWithRuleName()).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.Equal(t, "claude", getStr(t, a, ruleNameAttr))
	assert.InDelta(t, 0.010935, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

// Which of several overlapping patterns actually fired is the point of the
// feature: "gpt-4o*" and "*" both match, and the name proves the first won.
func TestRuleName_DisambiguatesOverlappingPatterns(t *testing.T) {
	cfg := *cfgWithRuleName()
	cfg.DefaultPricing = PricingConfig{Rules: []PricingRule{
		{Name: "gpt-4o", Pattern: []string{"gpt-4o*"}, In: 5.0, Out: 15.0},
		{Name: "fallback", Pattern: []string{"*"}, In: 1.0, Out: 2.0},
	}}

	for _, tc := range []struct{ model, wantRule string }{
		{"gpt-4o-2024-11-20", "gpt-4o"},
		{"some-other-model", "fallback"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			td := buildTrace(map[string]any{
				"gen_ai.request.model":      tc.model,
				"gen_ai.usage.input_tokens": int64(1000),
			})
			_, err := newProcessor(&cfg).ProcessTraces(context.Background(), td)
			require.NoError(t, err)
			assert.Equal(t, tc.wantRule, getStr(t, attrs(td), ruleNameAttr))
		})
	}
}

// Unconfigured destination key → nothing written, consistent with the other
// optional output attributes.
func TestRuleName_NotWrittenWhenKeyUnconfigured(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":      "gpt-4o",
		"gen_ai.usage.input_tokens": int64(1000),
	})

	// testCfg leaves OutputAttrs.RuleName empty.
	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get(ruleNameAttr)
	assert.False(t, ok, "rule name must not be written when the key is unconfigured")
}

// A rule with no name configured writes nothing rather than an empty string,
// since `name` is optional.
func TestRuleName_UnnamedRuleWritesNothing(t *testing.T) {
	cfg := *cfgWithRuleName()
	cfg.DefaultPricing = PricingConfig{Rules: []PricingRule{
		{Pattern: []string{"*"}, In: 1.0, Out: 2.0}, // no Name
	}}

	td := buildTrace(map[string]any{
		"gen_ai.request.model":      "gpt-4o",
		"gen_ai.usage.input_tokens": int64(1000),
	})

	_, err := newProcessor(&cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	_, ok := a.Get(ruleNameAttr)
	assert.False(t, ok, "an unnamed rule must not write an empty rule-name attribute")
	// The span is still priced.
	assert.InDelta(t, 1000*1.0/1e6, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

// A skipped span must not get a rule-name attribute.
func TestRuleName_NotWrittenWhenSpanSkipped(t *testing.T) {
	for name, spanAttrs := range map[string]map[string]any{
		"no_rule_match": {
			"gen_ai.request.model":      "unknown-model-xyz",
			"gen_ai.usage.input_tokens": int64(1000),
		},
		"zero_tokens": {
			"gen_ai.request.model": "gpt-4o",
		},
	} {
		t.Run(name, func(t *testing.T) {
			td := buildTrace(spanAttrs)
			_, err := newProcessor(cfgWithRuleName()).ProcessTraces(context.Background(), td)
			require.NoError(t, err)

			_, ok := attrs(td).Get(ruleNameAttr)
			assert.False(t, ok, "rule name must not be written for an unpriced span")
		})
	}
}
