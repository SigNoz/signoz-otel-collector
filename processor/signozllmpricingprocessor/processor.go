package signozllmpricingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor"

import (
	"context"
	"path"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// maxMatchCacheSize bounds matchCache; least recently seen model names are evicted beyond it.
const maxMatchCacheSize = 1000

// tokens are one span's per-bucket token counts.
type tokens struct {
	input      float64
	output     float64
	cacheRead  float64
	cacheWrite float64
}

// costs holds the computed per-bucket costs for a single span.
type costs struct {
	input      float64
	output     float64
	cacheRead  float64
	cacheWrite float64
	total      float64
}

// compiledRule is the hot-path form of PricingRule.
type compiledRule struct {
	name       string
	pattern    string
	cacheMode  CacheMode // "", CacheModeSubtract, or CacheModeAdditive
	in         float64
	out        float64
	cacheRead  float64
	cacheWrite float64
}

type llmCostProcessor struct {
	// Source attribute keys.
	modelAttr      string
	inAttr         string
	outAttr        string
	cacheReadAttr  string
	cacheWriteAttr string

	// Destination attribute keys. Empty string means "don't write".
	outInAttr         string
	outOutAttr        string
	outCacheReadAttr  string
	outCacheWriteAttr string
	outTotalAttr      string
	outTotalInputAttr string

	divisor float64 // 1e6 for per_million_tokens
	rules   []compiledRule

	// matchCache maps model -> *compiledRule; nil records that no rule matched.
	matchCache *lru.Cache[string, *compiledRule]
}

func newProcessor(cfg *Config) *llmCostProcessor {
	// Expand each rule's pattern list into separate compiled rules. This keeps
	// the match hot-path simple (one glob per entry) while preserving the
	// first-match-wins semantics across patterns within the same rule.
	rules := make([]compiledRule, 0, len(cfg.DefaultPricing.Rules))
	for _, r := range cfg.DefaultPricing.Rules {
		for _, p := range r.Pattern {
			rules = append(rules, compiledRule{
				name:       r.Name,
				pattern:    p,
				cacheMode:  r.Cache.Mode,
				in:         r.In,
				out:        r.Out,
				cacheRead:  r.Cache.Read,
				cacheWrite: r.Cache.Write,
			})
		}
	}

	divisor := 1e6 // UnitPerMillionTokens

	// lru.New errors only for a non-positive size.
	matchCache, _ := lru.New[string, *compiledRule](maxMatchCacheSize)

	return &llmCostProcessor{
		modelAttr:         cfg.Attrs.Model,
		inAttr:            cfg.Attrs.In,
		outAttr:           cfg.Attrs.Out,
		cacheReadAttr:     cfg.Attrs.CacheRead,
		cacheWriteAttr:    cfg.Attrs.CacheWrite,
		outInAttr:         cfg.OutputAttrs.In,
		outOutAttr:        cfg.OutputAttrs.Out,
		outCacheReadAttr:  cfg.OutputAttrs.CacheRead,
		outCacheWriteAttr: cfg.OutputAttrs.CacheWrite,
		outTotalAttr:      cfg.OutputAttrs.Total,
		outTotalInputAttr: cfg.OutputAttrs.TotalInputTokens,
		divisor:           divisor,
		rules:             rules,
		matchCache:        matchCache,
	}
}

// ProcessTraces computes LLM costs for every span that carries a model attribute
// matching a configured pricing rule. Cost attributes sent by the user are dropped
// first: the prefix is excluded from billing, so nothing user-supplied may live under it.
func (p *llmCostProcessor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rss.At(i).Resource().Attributes().RemoveIf(isReservedCostAttr)
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				attrs := spans.At(k).Attributes()
				attrs.RemoveIf(isReservedCostAttr)
				p.processSpan(attrs)
			}
		}
	}
	return td, nil
}

// processSpan finds the matching pricing rule for the span's model, computes
// costs, and writes them back as span attributes.
func (p *llmCostProcessor) processSpan(attrs pcommon.Map) {
	modelVal, ok := attrs.Get(p.modelAttr)
	if !ok {
		return
	}
	model := modelVal.Str()

	rule := p.matchRule(model)
	if rule == nil {
		return
	}

	raw := tokens{
		input:      getTokenCount(attrs, p.inAttr),
		output:     getTokenCount(attrs, p.outAttr),
		cacheRead:  getTokenCount(attrs, p.cacheReadAttr),
		cacheWrite: getTokenCount(attrs, p.cacheWriteAttr),
	}
	if raw == (tokens{}) {
		return
	}

	billed, totalInput := rule.normalize(raw)
	p.writeAttrs(attrs, p.price(rule, billed))
	putIntIfKey(attrs, p.outTotalInputAttr, int64(totalInput))
}

// matchRule returns the first rule whose pattern matches model, or nil.
func (p *llmCostProcessor) matchRule(model string) *compiledRule {
	if rule, ok := p.matchCache.Get(model); ok {
		return rule
	}

	var rule *compiledRule
	for i := range p.rules {
		if ok, _ := path.Match(p.rules[i].pattern, model); ok {
			rule = &p.rules[i]
			break
		}
	}

	p.matchCache.Add(model, rule)
	return rule
}

// normalize splits raw counters into the buckets the rule prices and counts every
// input token once.
func (r *compiledRule) normalize(raw tokens) (billed tokens, totalInput float64) {
	switch r.cacheMode {
	case CacheModeAdditive:
		// Additive mode (e.g. Anthropic): cache_read and cache_creation sit outside input_tokens,
		// so every bucket is billed and the total is their sum.
		return raw, raw.input + raw.cacheRead + raw.cacheWrite
	case CacheModeSubtract:
		// Subtract mode (e.g. OpenAI, Gemini): cache_read is a slice of input_tokens, so it is
		// moved out before the input rate applies, and cache writes are not priced per token.
		return tokens{input: max(raw.input-raw.cacheRead, 0), output: raw.output, cacheRead: raw.cacheRead}, raw.input
	default:
		// Unknown mode: input is taken as the whole input and the cache buckets are skipped.
		return tokens{input: raw.input, output: raw.output}, raw.input
	}
}

// price bills each bucket at the rule's per-million rate.
func (p *llmCostProcessor) price(rule *compiledRule, t tokens) costs {
	d := p.divisor
	c := costs{
		input:      t.input * rule.in / d,
		output:     t.output * rule.out / d,
		cacheRead:  t.cacheRead * rule.cacheRead / d,
		cacheWrite: t.cacheWrite * rule.cacheWrite / d,
	}
	c.total = c.input + c.output + c.cacheRead + c.cacheWrite
	return c
}

// writeAttrs writes the computed costs to the span attribute map.
// Fields with an empty destination key are skipped.
func (p *llmCostProcessor) writeAttrs(attrs pcommon.Map, c costs) {
	putIfKey(attrs, p.outInAttr, c.input)
	putIfKey(attrs, p.outOutAttr, c.output)
	putIfKey(attrs, p.outCacheReadAttr, c.cacheRead)
	putIfKey(attrs, p.outCacheWriteAttr, c.cacheWrite)
	putIfKey(attrs, p.outTotalAttr, c.total)
}

// getTokenCount reads a numeric attribute as float64. Returns 0 if absent or
// not a numeric type.
func getTokenCount(attrs pcommon.Map, key string) float64 {
	if key == "" {
		return 0
	}
	v, ok := attrs.Get(key)
	if !ok {
		return 0
	}
	switch v.Type() {
	case pcommon.ValueTypeInt:
		return float64(v.Int())
	case pcommon.ValueTypeDouble:
		return v.Double()
	}
	return 0
}

// putIfKey writes a float64 attribute only when key is non-empty.
func putIfKey(attrs pcommon.Map, key string, val float64) {
	if key != "" {
		attrs.PutDouble(key, val)
	}
}

func putIntIfKey(attrs pcommon.Map, key string, val int64) {
	if key != "" {
		attrs.PutInt(key, val)
	}
}

func isReservedCostAttr(key string, _ pcommon.Value) bool {
	return metering.ExcludeSigNozLLMPricingSpanAttrs.MatchString(key)
}
