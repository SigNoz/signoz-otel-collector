package signozllmpricingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor"

import (
	"context"
	"path"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

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
	additive   bool // true → CacheModeAdditive, false → CacheModeSubtract
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

	divisor float64 // 1e6 for per_million_tokens
	rules   []compiledRule
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
				additive:   r.Cache.Mode == CacheModeAdditive,
				in:         r.In,
				out:        r.Out,
				cacheRead:  r.Cache.Read,
				cacheWrite: r.Cache.Write,
			})
		}
	}

	divisor := 1e6 // UnitPerMillionTokens

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
		divisor:           divisor,
		rules:             rules,
	}
}

// ProcessTraces computes LLM costs for every span that carries a model attribute
// matching a configured pricing rule.
func (p *llmCostProcessor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.processSpan(spans.At(k).Attributes())
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

	in := getTokenCount(attrs, p.inAttr)
	out := getTokenCount(attrs, p.outAttr)
	cacheRead := getTokenCount(attrs, p.cacheReadAttr)
	cacheWrite := getTokenCount(attrs, p.cacheWriteAttr)

	if in == 0 && out == 0 && cacheRead == 0 && cacheWrite == 0 {
		return
	}

	c := p.compute(rule, in, out, cacheRead, cacheWrite)
	p.writeAttrs(attrs, c)
}

// matchRule returns the first rule whose pattern matches model, or nil.
func (p *llmCostProcessor) matchRule(model string) *compiledRule {
	for i := range p.rules {
		if ok, _ := path.Match(p.rules[i].pattern, model); ok {
			return &p.rules[i]
		}
	}
	return nil
}

// compute calculates per-bucket and total costs.
//
// subtract mode (e.g. OpenAI): cache_read tokens are already counted inside
// input_tokens, so they are subtracted before billing the regular input rate.
//
//	billed_input = max(input_tokens - cache_read, 0)
//	cost_input   = billed_input  * price_in        / divisor
//	cost_cache_read  = cache_read    * price_cache_read / divisor
//	cost_output  = output_tokens * price_out       / divisor
//	total        = cost_input + cost_cache_read + cost_output
//
// additive mode (e.g. Anthropic): cache_read/write are separate from
// input_tokens; all four buckets are billed independently.
//
//	cost_input       = input_tokens  * price_in         / divisor
//	cost_cache_read  = cache_read    * price_cache_read  / divisor
//	cost_cache_write = cache_write   * price_cache_write / divisor
//	cost_output      = output_tokens * price_out         / divisor
//	total            = cost_input + cost_cache_read + cost_cache_write + cost_output
func (p *llmCostProcessor) compute(rule *compiledRule, in, out, cacheRead, cacheWrite float64) costs {
	d := p.divisor
	var c costs

	if rule.additive {
		c.input = in * rule.in / d
		c.cacheRead = cacheRead * rule.cacheRead / d
		c.cacheWrite = cacheWrite * rule.cacheWrite / d
		c.output = out * rule.out / d
	} else {
		billedInput := in - cacheRead
		if billedInput < 0 {
			billedInput = 0
		}
		c.input = billedInput * rule.in / d
		c.cacheRead = cacheRead * rule.cacheRead / d
		c.cacheWrite = 0 // not billed in subtract mode
		c.output = out * rule.out / d
	}

	c.total = c.input + c.cacheRead + c.cacheWrite + c.output
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
