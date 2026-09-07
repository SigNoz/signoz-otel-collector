package signozllmpricingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor"

import (
	"context"
	"path"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor/internal/metadata"
)

// skipReason enumerates the ways a span can end up unpriced. Every early return
// in processSpan maps to exactly one of these, so an unpriced span is always
// counted and never silently dropped.
type skipReason int

const (
	// reasonMissingModelAttr — the configured model attribute key is absent.
	reasonMissingModelAttr skipReason = iota
	// reasonNoRuleMatch — the model value matched no configured rule pattern.
	reasonNoRuleMatch
	// reasonZeroTokens — all four token counts read as zero.
	reasonZeroTokens
	// reasonNonNumericToken — at least one token attribute was present but held
	// a non-numeric value (e.g. a string), and no usable count remained.
	reasonNonNumericToken

	numSkipReasons
)

var skipReasonNames = [numSkipReasons]string{
	reasonMissingModelAttr: "missing_model_attr",
	reasonNoRuleMatch:      "no_rule_match",
	reasonZeroTokens:       "zero_tokens",
	reasonNonNumericToken:  "non_numeric_token_value",
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

	divisor float64 // 1e6 for per_million_tokens
	rules   []compiledRule

	telemetryBuilder *metadata.TelemetryBuilder

	// pricedAttrs and unpricedAttrs are precomputed so the hot path performs no
	// attribute-set allocation per span; unpricedAttrs is indexed by skipReason.
	pricedAttrs   metric.MeasurementOption
	unpricedAttrs [numSkipReasons]metric.MeasurementOption
}

func newProcessor(cfg *Config, set processor.Settings) (*llmCostProcessor, error) {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
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

	// Precompute one attribute set per outcome. Built once at startup so
	// ProcessTraces adds no per-span allocation for telemetry.
	baseAttrs := []attribute.KeyValue{
		attribute.String("processor", set.ID.String()),
		attribute.String("otel.signal", "traces"),
	}
	var unpricedAttrs [numSkipReasons]metric.MeasurementOption
	for r := range numSkipReasons {
		unpricedAttrs[r] = metric.WithAttributeSet(attribute.NewSet(
			append(append([]attribute.KeyValue{}, baseAttrs...),
				attribute.String("reason", skipReasonNames[r]))...,
		))
	}

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
		telemetryBuilder:  tb,
		pricedAttrs:       metric.WithAttributeSet(attribute.NewSet(baseAttrs...)),
		unpricedAttrs:     unpricedAttrs,
	}, nil
}

// ProcessTraces computes LLM costs for every span that carries a model attribute
// matching a configured pricing rule.
func (p *llmCostProcessor) ProcessTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.processSpan(ctx, spans.At(k).Attributes())
			}
		}
	}
	return td, nil
}

// recordSkip counts a span that was left unpriced, tagged with why.
func (p *llmCostProcessor) recordSkip(ctx context.Context, reason skipReason) {
	p.telemetryBuilder.ProcessorSignozllmpricingUnpricedSpans.Add(ctx, 1, p.unpricedAttrs[reason])
}

// processSpan finds the matching pricing rule for the span's model, computes
// costs, and writes them back as span attributes.
func (p *llmCostProcessor) processSpan(ctx context.Context, attrs pcommon.Map) {
	modelVal, ok := attrs.Get(p.modelAttr)
	if !ok {
		p.recordSkip(ctx, reasonMissingModelAttr)
		return
	}
	model := modelVal.Str()

	rule := p.matchRule(model)
	if rule == nil {
		p.recordSkip(ctx, reasonNoRuleMatch)
		return
	}

	in, inBad := getTokenCount(attrs, p.inAttr)
	out, outBad := getTokenCount(attrs, p.outAttr)
	cacheRead, cacheReadBad := getTokenCount(attrs, p.cacheReadAttr)
	cacheWrite, cacheWriteBad := getTokenCount(attrs, p.cacheWriteAttr)

	if in == 0 && out == 0 && cacheRead == 0 && cacheWrite == 0 {
		// Distinguish "genuinely zero/absent counts" from "counts were present
		// but unreadable" — the latter is a data-shape bug worth surfacing
		// separately, and previously vanished into the same silent return.
		if inBad || outBad || cacheReadBad || cacheWriteBad {
			p.recordSkip(ctx, reasonNonNumericToken)
		} else {
			p.recordSkip(ctx, reasonZeroTokens)
		}
		return
	}

	c := p.compute(rule, in, out, cacheRead, cacheWrite)
	p.writeAttrs(attrs, c)
	p.telemetryBuilder.ProcessorSignozllmpricingPricedSpans.Add(ctx, 1, p.pricedAttrs)
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
	c.output = out * rule.out / d

	switch rule.cacheMode {
	case CacheModeAdditive:
		c.input = in * rule.in / d
		c.cacheRead = cacheRead * rule.cacheRead / d
		c.cacheWrite = cacheWrite * rule.cacheWrite / d
	case CacheModeSubtract:
		billedInput := in - cacheRead
		if billedInput < 0 {
			billedInput = 0
		}
		c.input = billedInput * rule.in / d
		c.cacheRead = cacheRead * rule.cacheRead / d
	default:
		// Unknown/absent mode: we don't know how cache_read relates to
		// input_tokens, so bill input as-is and don't bill cache.
		c.input = in * rule.in / d
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

// getTokenCount reads a numeric attribute as float64. It returns 0 if the key
// is unconfigured or the attribute is absent. The second return value reports
// whether the attribute was present but held a non-numeric value (e.g. a token
// count that arrived as a string), which the caller surfaces as its own skip
// reason rather than letting it masquerade as a zero count.
func getTokenCount(attrs pcommon.Map, key string) (value float64, malformed bool) {
	if key == "" {
		return 0, false
	}
	v, ok := attrs.Get(key)
	if !ok {
		return 0, false
	}
	switch v.Type() {
	case pcommon.ValueTypeInt:
		return float64(v.Int()), false
	case pcommon.ValueTypeDouble:
		return v.Double(), false
	}
	return 0, true
}

// putIfKey writes a float64 attribute only when key is non-empty.
func putIfKey(attrs pcommon.Map, key string, val float64) {
	if key != "" {
		attrs.PutDouble(key, val)
	}
}
