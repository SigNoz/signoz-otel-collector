package signozllmpricingprocessor

import (
	"context"
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
		Unit: UnitPerMillionTokens,
		Rules: []PricingRule{
			{
				Name:    "gpt-4o",
				Pattern: []string{"gpt-4o*"},
				Cache: PricingRuleCache{
					Mode: CacheModeSubtract,
					Read: 2.5,
				},
				In:  5.0,
				Out: 15.0,
			},
			{
				Name:    "claude",
				Pattern: []string{"claude-*"},
				Cache: PricingRuleCache{
					Mode:  CacheModeAdditive,
					Read:  0.30,
					Write: 3.75,
				},
				In:  3.0,
				Out: 15.0,
			},
		},
	},
	OutputAttrs: OutputMapping{
		In:         "_signoz.gen_ai.cost_input",
		Out:        "_signoz.gen_ai.cost_output",
		CacheRead:  "_signoz.gen_ai.cost_cache_read",
		CacheWrite: "_signoz.gen_ai.cost_cache_write",
		Total:      "_signoz.gen_ai.total_cost",
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

func TestSubtractMode_NoCaching(t *testing.T) {
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
	assert.InDelta(t, 0.005, getDouble(t, a, "_signoz.gen_ai.cost_input"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "_signoz.gen_ai.cost_output"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "_signoz.gen_ai.cost_cache_read"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "_signoz.gen_ai.cost_cache_write"), 1e-9)
	assert.InDelta(t, 0.0125, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

func TestSubtractMode_WithCacheRead(t *testing.T) {
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
	assert.InDelta(t, 0.004, getDouble(t, a, "_signoz.gen_ai.cost_input"), 1e-9)
	assert.InDelta(t, 0.0005, getDouble(t, a, "_signoz.gen_ai.cost_cache_read"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "_signoz.gen_ai.cost_output"), 1e-9)
	assert.InDelta(t, 0.012, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

func TestSubtractMode_CacheReadExceedsInput(t *testing.T) {
	// cache_read > input → billed_input clamped to 0
	td := buildTrace(map[string]any{
		"gen_ai.request.model":           "gpt-4o",
		"gen_ai.usage.input_tokens":      int64(100),
		"gen_ai.usage.output_tokens":     int64(200),
		"gen_ai.usage.cache_read_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 0.0, getDouble(t, a, "_signoz.gen_ai.cost_input"), 1e-9)
	// cache_read cost still billed for the actual cache_read value
	assert.InDelta(t, 500*2.5/1e6, getDouble(t, a, "_signoz.gen_ai.cost_cache_read"), 1e-9)
}

func TestAdditiveMode(t *testing.T) {
	// 1000 input, 500 output, 200 cache_read, 100 cache_write
	// cost_input       = 1000 * 3.0  / 1e6 = 0.003
	// cost_output      = 500  * 15.0 / 1e6 = 0.0075
	// cost_cache_read  = 200  * 0.30 / 1e6 = 0.00006
	// cost_cache_write = 100  * 3.75 / 1e6 = 0.000375
	// total = 0.010935
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
	assert.InDelta(t, 0.003, getDouble(t, a, "_signoz.gen_ai.cost_input"), 1e-9)
	assert.InDelta(t, 0.0075, getDouble(t, a, "_signoz.gen_ai.cost_output"), 1e-9)
	assert.InDelta(t, 0.00006, getDouble(t, a, "_signoz.gen_ai.cost_cache_read"), 1e-9)
	assert.InDelta(t, 0.000375, getDouble(t, a, "_signoz.gen_ai.cost_cache_write"), 1e-9)
	assert.InDelta(t, 0.010935, getDouble(t, a, "_signoz.gen_ai.total_cost"), 1e-9)
}

func TestAdditiveMode_NoCaching(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "claude-3-haiku",
		"gen_ai.usage.input_tokens":  int64(2000),
		"gen_ai.usage.output_tokens": int64(1000),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	assert.InDelta(t, 2000*3.0/1e6, getDouble(t, a, "_signoz.gen_ai.cost_input"), 1e-9)
	assert.InDelta(t, 1000*15.0/1e6, getDouble(t, a, "_signoz.gen_ai.cost_output"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "_signoz.gen_ai.cost_cache_read"), 1e-9)
	assert.InDelta(t, 0.0, getDouble(t, a, "_signoz.gen_ai.cost_cache_write"), 1e-9)
}

func TestRuleFirstMatchWins(t *testing.T) {
	// "gpt-4o*" should match before a catch-all "*" if one existed.
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o-2024-11-20",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(0),
	})
	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)
	// gpt-4o rule price_in=5.0
	assert.InDelta(t, 1000*5.0/1e6, getDouble(t, attrs(td), "_signoz.gen_ai.cost_input"), 1e-9)
}

func TestNoMatchingRule_SkipsSpan(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "unknown-model-xyz",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("_signoz.gen_ai.total_cost")
	assert.False(t, ok, "no cost attrs must be written when no rule matches")
}

func TestNoModelAttr_SkipsSpan(t *testing.T) {
	td := buildTrace(map[string]any{
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("_signoz.gen_ai.total_cost")
	assert.False(t, ok)
}

func TestAllTokensZero_SkipsSpan(t *testing.T) {
	// Model matches a rule but no token attrs present → nothing written.
	td := buildTrace(map[string]any{
		"gen_ai.request.model": "gpt-4o",
	})

	_, err := newProcessor(testCfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := attrs(td).Get("_signoz.gen_ai.total_cost")
	assert.False(t, ok, "cost attrs must not be written when all token counts are zero")
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

	assert.InDelta(t, 500*5.0/1e6, getDouble(t, attrs(td), "_signoz.gen_ai.cost_input"), 1e-9)
}

func TestOptionalOutputAttrs(t *testing.T) {
	cfg := *testCfg
	cfg.OutputAttrs = OutputMapping{
		Total: "_signoz.gen_ai.total_cost", // only total
	}

	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	_, err := newProcessor(&cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	a := attrs(td)
	_, hasIn := a.Get("_signoz.gen_ai.cost_input")
	assert.False(t, hasIn, "per-bucket attrs must not be written when key is empty")

	_, hasTotal := a.Get("_signoz.gen_ai.total_cost")
	assert.True(t, hasTotal)
}

func TestComputeSubtract(t *testing.T) {
	p := newProcessor(testCfg)
	rule := &p.rules[0] // gpt-4o*: subtract

	c := p.compute(rule, 1000, 500, 200, 0)
	assert.InDelta(t, 800*5.0/1e6, c.input, 1e-9)
	assert.InDelta(t, 200*2.5/1e6, c.cacheRead, 1e-9)
	assert.InDelta(t, 0.0, c.cacheWrite, 1e-9)
	assert.InDelta(t, 500*15.0/1e6, c.output, 1e-9)
	assert.InDelta(t, c.input+c.cacheRead+c.output, c.total, 1e-9)
}

func TestComputeAdditive(t *testing.T) {
	p := newProcessor(testCfg)
	rule := &p.rules[1] // claude-*: additive

	c := p.compute(rule, 1000, 500, 200, 100)
	assert.InDelta(t, 1000*3.0/1e6, c.input, 1e-9)
	assert.InDelta(t, 200*0.30/1e6, c.cacheRead, 1e-9)
	assert.InDelta(t, 100*3.75/1e6, c.cacheWrite, 1e-9)
	assert.InDelta(t, 500*15.0/1e6, c.output, 1e-9)
	assert.InDelta(t, c.input+c.cacheRead+c.cacheWrite+c.output, c.total, 1e-9)
}
