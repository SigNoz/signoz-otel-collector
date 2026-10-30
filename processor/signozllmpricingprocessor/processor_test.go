package signozllmpricingprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var testCfg = &Config{
	Attrs: AttrMapping{
		Model:      "gen_ai.request.model",
		In:         "gen_ai.usage.input_tokens",
		Out:        "gen_ai.usage.output_tokens",
		CacheRead:  "gen_ai.usage.cache_read_tokens",
		CacheWrite: "gen_ai.usage.cache_write_tokens",
	},
	DefaultPricing: PricingConfig{
		Rules: []PricingRule{
			{
				Name:    "gpt-4o",
				Pattern: []string{"gpt-4o*"},
				Cache: PricingRuleCache{
					Read: 2.5,
				},
				In:  5.0,
				Out: 15.0,
			},
			{
				Name:    "claude",
				Pattern: []string{"claude-*"},
				Cache: PricingRuleCache{
					Read:  0.30,
					Write: 3.75,
				},
				In:  3.0,
				Out: 15.0,
			},
		},
	},
	OutputAttrs: OutputMapping{
		In:         "signoz.gen_ai.usage.input_tokens.cost",
		Out:        "signoz.gen_ai.usage.output_tokens.cost",
		CacheRead:  "signoz.gen_ai.usage.cache_read.input_tokens.cost",
		CacheWrite: "signoz.gen_ai.usage.cache_write.input_tokens.cost",
		Total:      "signoz.gen_ai.usage.tokens.cost",
	},
}

func buildTrace(spanAttrs map[string]any) ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	for k, v := range spanAttrs {
		switch val := v.(type) {
		case int:
			span.Attributes().PutInt(k, int64(val))
		case int64:
			span.Attributes().PutInt(k, val)
		case float64:
			span.Attributes().PutDouble(k, val)
		case string:
			span.Attributes().PutStr(k, val)
		}
	}
	return td
}

func attrs(td ptrace.Traces) pcommon.Map {
	return td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
}

func getDouble(t *testing.T, m pcommon.Map, key string) float64 {
	t.Helper()
	v, ok := m.Get(key)
	require.True(t, ok, "expected attribute %q to be present", key)
	return v.Double()
}

func TestNoCaching(t *testing.T) {
	// 1000 input, 500 output, no cache
	// cost_input = 1000 * 5.0 / 1e6 = 0.005
	// cost_output = 500 * 15.0 / 1e6 = 0.0075
	// total = 0.0125
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 0.005, getDouble(t, a, "signoz.gen_ai.usage.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "signoz.gen_ai.usage.output_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "signoz.gen_ai.usage.cache_read.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "signoz.gen_ai.usage.cache_write.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0125, getDouble(t, a, "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestCacheReadWithinInput(t *testing.T) {
	// 1000 input (includes 200 cache_read), 500 output
	// billed_input = 1000 - 200 = 800
	// cost_input     = 800  * 5.0  / 1e6 = 0.004
	// cost_cache_read = 200 * 2.5  / 1e6 = 0.0005
	// cost_output    = 500  * 15.0 / 1e6 = 0.0075
	// total = 0.012
	td := buildTrace(map[string]any{
		"gen_ai.request.model":           "gpt-4o-mini",
		"gen_ai.usage.input_tokens":      int64(1000),
		"gen_ai.usage.output_tokens":     int64(500),
		"gen_ai.usage.cache_read_tokens": int64(200),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 0.004, getDouble(t, a, "signoz.gen_ai.usage.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0005, getDouble(t, a, "signoz.gen_ai.usage.cache_read.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "signoz.gen_ai.usage.output_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.012, getDouble(t, a, "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestCacheExceedsInput(t *testing.T) {
	// cache_read > input cannot be a subset, so input is billed whole alongside the cache.
	// cost_input      = 100 * 5.0 / 1e6 = 0.0005
	// cost_cache_read = 500 * 2.5 / 1e6 = 0.00125
	// cost_output     = 200 * 15.0 / 1e6 = 0.003
	td := buildTrace(map[string]any{
		"gen_ai.request.model":           "gpt-4o",
		"gen_ai.usage.input_tokens":      int64(100),
		"gen_ai.usage.output_tokens":     int64(200),
		"gen_ai.usage.cache_read_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 0.0005, getDouble(t, a, "signoz.gen_ai.usage.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.00125, getDouble(t, a, "signoz.gen_ai.usage.cache_read.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.003, getDouble(t, a, "signoz.gen_ai.usage.output_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.00475, getDouble(t, a, "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestCacheReadAndWriteWithinInput(t *testing.T) {
	// 1000 input (includes 200 cache_read and 100 cache_write), 500 output
	// billed_input     = 1000 - 200 - 100 = 700
	// cost_input       = 700 * 3.0  / 1e6 = 0.0021
	// cost_output      = 500 * 15.0 / 1e6 = 0.0075
	// cost_cache_read  = 200 * 0.30 / 1e6 = 0.00006
	// cost_cache_write = 100 * 3.75 / 1e6 = 0.000375
	// total = 0.010035
	td := buildTrace(map[string]any{
		"gen_ai.request.model":            "claude-3-5-sonnet",
		"gen_ai.usage.input_tokens":       int64(1000),
		"gen_ai.usage.output_tokens":      int64(500),
		"gen_ai.usage.cache_read_tokens":  int64(200),
		"gen_ai.usage.cache_write_tokens": int64(100),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 0.0021, getDouble(t, a, "signoz.gen_ai.usage.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "signoz.gen_ai.usage.output_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.00006, getDouble(t, a, "signoz.gen_ai.usage.cache_read.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.000375, getDouble(t, a, "signoz.gen_ai.usage.cache_write.input_tokens.cost"), 1e-9)
	assert.InDelta(t, 0.010035, getDouble(t, a, "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestMatchRule(t *testing.T) {
	gpt5 := PricingRule{Name: "gpt-5", Pattern: []string{"openai/gpt-5", "gpt-5"}, In: 1, Out: 1}
	gpt5Mini := PricingRule{Name: "gpt-5-mini", Pattern: []string{"openai/gpt-5-mini", "gpt-5-mini"}, In: 1, Out: 1}
	opus5 := PricingRule{Name: "claude-opus-5", Pattern: []string{"anthropic/claude-opus-5", "claude-opus-5"}, In: 5, Out: 25}
	opus55 := PricingRule{Name: "claude-opus-5.5", Pattern: []string{"anthropic/claude-opus-5.5", "claude-opus-5.5", "claude-opus-5-5"}, In: 4, Out: 20}
	grok := PricingRule{Name: "grok-build-0.1", Pattern: []string{"x-ai/grok-build-0.1", "grok-build-0.1"}, In: 1, Out: 2}

	testCases := []struct {
		name  string
		rules []PricingRule
		model string
		want  string
	}{
		{name: "ExactModel", rules: []PricingRule{grok}, model: "grok-build-0.1", want: "grok-build-0.1"},
		{name: "ProviderQualifiedModel", rules: []PricingRule{grok}, model: "x-ai/grok-build-0.1", want: "grok-build-0.1"},
		{name: "DatedSnapshot_Matches", rules: []PricingRule{grok}, model: "grok-build-0.1-20260520", want: "grok-build-0.1"},
		{name: "DashedDate_Matches", rules: []PricingRule{gpt5}, model: "gpt-5-2025-08-07", want: "gpt-5"},
		{name: "MonthYearDate_Matches", rules: []PricingRule{{Name: "command-a", Pattern: []string{"command-a"}, In: 1, Out: 1}}, model: "command-a-03-2025", want: "command-a"},
		{name: "StableVersion_Matches", rules: []PricingRule{{Name: "gemini-2.0-flash", Pattern: []string{"gemini-2.0-flash"}, In: 1, Out: 1}}, model: "gemini-2.0-flash-001", want: "gemini-2.0-flash"},
		{name: "LatestAlias_Matches", rules: []PricingRule{{Name: "claude-3.5-sonnet", Pattern: []string{"claude-3-5-sonnet"}, In: 1, Out: 1}}, model: "claude-3-5-sonnet-latest", want: "claude-3.5-sonnet"},
		{name: "BedrockVersion_Matches", rules: []PricingRule{{Name: "claude-sonnet-4.5", Pattern: []string{"claude-sonnet-4-5"}, In: 1, Out: 1}}, model: "claude-sonnet-4-5-20250929-v1:0", want: "claude-sonnet-4.5"},
		{name: "VertexVersion_Matches", rules: []PricingRule{{Name: "claude-opus-4.5", Pattern: []string{"claude-opus-4-5"}, In: 1, Out: 1}}, model: "claude-opus-4-5@20251101", want: "claude-opus-4.5"},
		{name: "HyphenAlias_PricesSDKSpelling", rules: []PricingRule{opus5, opus55}, model: "claude-opus-5-5", want: "claude-opus-5.5"},
		{name: "HyphenAlias_PricesDatedSDKSpelling", rules: []PricingRule{opus5, opus55}, model: "claude-opus-5-5-20260921", want: "claude-opus-5.5"},
		{name: "MinorVersionBump_NoRuleYet_NoMatch", rules: []PricingRule{opus5}, model: "claude-opus-5-5", want: ""},
		{name: "DottedVersionBump_NoMatch", rules: []PricingRule{gpt5}, model: "gpt-5.1-2025-11-13", want: ""},
		{name: "SiblingModel_NoMatch", rules: []PricingRule{gpt5}, model: "gpt-5-mini-2025-08-07", want: ""},
		{name: "ExtraDigitVersion_NoMatch", rules: []PricingRule{grok}, model: "grok-build-0.10", want: ""},
		{name: "ShorterThanPrefix_NoMatch", rules: []PricingRule{grok}, model: "grok-build-0", want: ""},
		{name: "DifferentCase_NoMatch", rules: []PricingRule{grok}, model: "Grok-Build-0.1", want: ""},
		{name: "OverlappingPrefixes_LongestWins", rules: []PricingRule{gpt5, gpt5Mini}, model: "gpt-5-mini-2025-08-07", want: "gpt-5-mini"},
		{name: "TrailingStar_MatchesAnySuffix", rules: []PricingRule{{Name: "gpt-4o", Pattern: []string{"gpt-4o*"}, In: 1, Out: 1}}, model: "gpt-4o-mini", want: "gpt-4o"},
		{name: "StarOnly_MatchesEverything", rules: []PricingRule{{Name: "any", Pattern: []string{"*"}, In: 1, Out: 1}}, model: "mystery-model", want: "any"},
		{name: "DuplicatePrefix_ConfigOrderWins", rules: []PricingRule{{Name: "first", Pattern: []string{"o3"}, In: 1, Out: 1}, {Name: "second", Pattern: []string{"o3"}, In: 1, Out: 1}}, model: "o3-2025-04-16", want: "first"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := *testCfg
			cfg.DefaultPricing = PricingConfig{Rules: testCase.rules}

			rule := newProcessor(&cfg).matchRule(testCase.model)
			if testCase.want == "" {
				assert.Nil(t, rule)
				return
			}
			require.NotNil(t, rule)
			assert.Equal(t, testCase.want, rule.name)
		})
	}
}

func TestNoMatchingRule_SkipsSpan(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "unknown-model-xyz",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("signoz.gen_ai.usage.tokens.cost")
	assert.False(t, ok, "no cost attrs must be written when no rule matches")
}

func TestNoModelAttr_SkipsSpan(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("signoz.gen_ai.usage.tokens.cost")
	assert.False(t, ok)
}

func TestAllTokensZero_SkipsSpan(t *testing.T) {
	// Model matches a rule but no token attrs present → nothing written.
	td := buildTrace(map[string]any{
		"gen_ai.request.model": "gpt-4o",
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("signoz.gen_ai.usage.tokens.cost")
	assert.False(t, ok, "cost attrs must not be written when all token counts are zero")
}

func TestDropsUserSentCostAttrs(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.usage.input_tokens":       int64(1000),
		"signoz.gen_ai.usage.tokens.cost": 42.0,
		"signoz.gen_ai.anything":          "smuggled",
	})
	td.ResourceSpans().At(0).Resource().Attributes().PutDouble("signoz.gen_ai.usage.tokens.cost", 42.0)

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("signoz.gen_ai.usage.tokens.cost")
	assert.False(t, ok)
	_, ok = attrs(td).Get("signoz.gen_ai.anything")
	assert.False(t, ok)
	_, ok = td.ResourceSpans().At(0).Resource().Attributes().Get("signoz.gen_ai.usage.tokens.cost")
	assert.False(t, ok)
	_, ok = attrs(td).Get("gen_ai.usage.input_tokens")
	assert.True(t, ok)
}

func TestUserSentCostReplacedByComputed(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":            "gpt-4o",
		"gen_ai.usage.input_tokens":       int64(1000),
		"gen_ai.usage.output_tokens":      int64(0),
		"signoz.gen_ai.usage.tokens.cost": 42.0,
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	assert.InDelta(t, 0.005, getDouble(t, attrs(td), "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestTokenAsFloat(t *testing.T) {
	// Token counts stored as double (some SDKs may emit floats).
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  float64(500),
		"gen_ai.usage.output_tokens": float64(250),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	assert.InDelta(t, 500*5.0/1e6, getDouble(t, attrs(td), "signoz.gen_ai.usage.input_tokens.cost"), 1e-9)
}

func TestOptionalOutputAttrs(t *testing.T) {
	cfg := *testCfg
	cfg.OutputAttrs = OutputMapping{
		Total: "signoz.gen_ai.usage.tokens.cost", // only total
	}

	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(&cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	_, hasIn := a.Get("signoz.gen_ai.usage.input_tokens.cost")
	assert.False(t, hasIn, "per-bucket attrs must not be written when key is empty")

	_, hasTotal := a.Get("signoz.gen_ai.usage.tokens.cost")
	assert.True(t, hasTotal)
}

func TestRuleWithoutCachePrices(t *testing.T) {
	cfg := *testCfg
	cfg.DefaultPricing = PricingConfig{Rules: []PricingRule{{Pattern: []string{"*"}, In: 1.0, Out: 2.0}}}

	td := buildTrace(map[string]any{
		"gen_ai.request.model":           "mystery-model",
		"gen_ai.usage.input_tokens":      int64(1000),
		"gen_ai.usage.cache_read_tokens": int64(200),
	})

	_, err := newProcessor(&cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 800*1.0/1e6, getDouble(t, a, "signoz.gen_ai.usage.tokens.cost"), 1e-9)
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name   string
		raw    tokens
		billed tokens
	}{
		{
			name:   "cache within input",
			raw:    tokens{input: 1000, output: 500, cacheRead: 200, cacheWrite: 100},
			billed: tokens{input: 700, output: 500, cacheRead: 200, cacheWrite: 100},
		},
		{
			name:   "cache equals input",
			raw:    tokens{input: 2048, output: 503, cacheRead: 1800, cacheWrite: 248},
			billed: tokens{input: 0, output: 503, cacheRead: 1800, cacheWrite: 248},
		},
		{
			name:   "cache exceeds input",
			raw:    tokens{input: 200, output: 150, cacheRead: 800, cacheWrite: 100},
			billed: tokens{input: 200, output: 150, cacheRead: 800, cacheWrite: 100},
		},
		{
			name:   "no cache",
			raw:    tokens{input: 1000, output: 500},
			billed: tokens{input: 1000, output: 500},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.billed, normalize(tc.raw))
		})
	}
}

func TestPrice(t *testing.T) {
	p := newProcessor(testCfg)
	rule := p.matchRule("claude-3-5-sonnet") // 3.0 in, 15.0 out, 0.30 read, 3.75 write
	require.NotNil(t, rule)

	c := p.price(rule, tokens{input: 1000, output: 500, cacheRead: 200, cacheWrite: 100})
	assert.InDelta(t, 1000*3.0/1e6, c.input, 1e-9)
	assert.InDelta(t, 500*15.0/1e6, c.output, 1e-9)
	assert.InDelta(t, 200*0.30/1e6, c.cacheRead, 1e-9)
	assert.InDelta(t, 100*3.75/1e6, c.cacheWrite, 1e-9)
	assert.InDelta(t, c.input+c.output+c.cacheRead+c.cacheWrite, c.total, 1e-9)
}

func TestMatchRuleCachesResult(t *testing.T) {
	p := newProcessor(testCfg)

	matched := p.matchRule("gpt-4o-mini")
	require.NotNil(t, matched)
	assert.Nil(t, p.matchRule("unpriced-model"))

	// Without rules, only cached results can be returned.
	p.rules = nil
	assert.Same(t, matched, p.matchRule("gpt-4o-mini"))
	assert.Nil(t, p.matchRule("unpriced-model"))

	rule, ok := p.matchCache.Get("unpriced-model")
	assert.True(t, ok)
	assert.Nil(t, rule)
}

func BenchmarkProcessTraces(b *testing.B) {
	const usedModels = 25

	for _, n := range []int{1000, 5000} {
		cfg := *testCfg
		cfg.DefaultPricing.Rules = make([]PricingRule, n)
		for i := range cfg.DefaultPricing.Rules {
			cfg.DefaultPricing.Rules[i] = PricingRule{Name: fmt.Sprintf("m-%d", i), Pattern: []string{fmt.Sprintf("vendor-%d/model-%d-*", i%20, i)}, In: 1, Out: 1}
		}
		p := newProcessor(&cfg)

		// Used models are spread evenly across the rule list, plus one with no rule.
		models := make([]string, 0, usedModels+1)
		for i := 0; i < usedModels; i++ {
			r := i * n / usedModels
			models = append(models, fmt.Sprintf("vendor-%d/model-%d-2025", r%20, r))
		}
		models = append(models, "unpriced-model")

		td := ptrace.NewTraces()
		spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
		for i := 0; i < 100; i++ {
			a := spans.AppendEmpty().Attributes()
			a.PutStr("gen_ai.request.model", models[i%len(models)])
			a.PutInt("gen_ai.usage.input_tokens", 1000)
			a.PutInt("gen_ai.usage.output_tokens", 500)
		}

		b.Run(fmt.Sprintf("rules=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = p.ProcessTraces(context.Background(), td)
			}
		})

		b.Run(fmt.Sprintf("parallel/rules=%d", n), func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				td := ptrace.NewTraces()
				spans.CopyTo(td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans())
				for pb.Next() {
					_, _ = p.ProcessTraces(context.Background(), td)
				}
			})
		})
	}
}
