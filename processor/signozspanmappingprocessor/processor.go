package signozspanmappingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozspanmappingprocessor"

import (
	"context"
	"path"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// parsedKey is a pre-parsed attribute source key. All string parsing happens
// once at construction time so the hot path does zero string operations.
type parsedKey struct {
	bare       string // key without any prefix
	isResource bool   // true → read from resource attributes
}

// parsedRule is the hot-path-ready form of AttributeRule.
type parsedRule struct {
	target          string
	writeToResource bool // context == ContextResource
	sources         []parsedKey
	move            bool // action == ActionMove
}

// parsedGroup is the hot-path-ready form of Group.
type parsedGroup struct {
	// Glob patterns split by where they are matched.
	attrPatterns []string // matched against span/log attribute keys
	resPatterns  []string // matched against resource attribute keys
	rules        []parsedRule
}

type aiProcessor struct {
	groups []parsedGroup
}

// newProcessor pre-parses the configuration so that the hot path (per span/log)
// performs no string allocation or parsing.
func newProcessor(cfg *Config) *aiProcessor {
	groups := make([]parsedGroup, len(cfg.Groups))
	for i, g := range cfg.Groups {
		pg := parsedGroup{
			attrPatterns: append([]string(nil), g.ExistsAny.Attributes...),
			resPatterns:  append([]string(nil), g.ExistsAny.Resource...),
			rules:        make([]parsedRule, len(g.Attributes)),
		}
		for j, rule := range g.Attributes {
			pr := parsedRule{
				target:          rule.Target,
				writeToResource: rule.Context == ContextResource,
				sources:         make([]parsedKey, len(rule.Sources)),
				move:            rule.Action == ActionMove,
			}
			for k, src := range rule.Sources {
				bare, isRes := isResourceKey(src)
				pr.sources[k] = parsedKey{bare: bare, isResource: isRes}
			}
			pg.rules[j] = pr
		}
		groups[i] = pg
	}
	return &aiProcessor{groups: groups}
}

// ProcessTraces applies attribute mappings to every span in the batch.
func (p *aiProcessor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resourceAttrs := rs.Resource().Attributes()
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.applyGroups(spans.At(k).Attributes(), resourceAttrs)
			}
		}
	}
	return td, nil
}

// applyGroups iterates groups and applies the rules of each group whose
// exists_any condition is satisfied.
func (p *aiProcessor) applyGroups(attrs, resourceAttrs pcommon.Map) {
	for i := range p.groups {
		g := &p.groups[i]
		if !conditionMet(g, attrs, resourceAttrs) {
			continue
		}
		for j := range g.rules {
			applyRule(&g.rules[j], attrs, resourceAttrs)
		}
	}
}

// conditionMet returns true when at least one attribute key in attrs or
// resourceAttrs matches any of the glob patterns in the group. It iterates the
// attribute maps only when there are patterns to check, and short-circuits as
// soon as a match is found.
func conditionMet(g *parsedGroup, attrs, resourceAttrs pcommon.Map) bool {
	if len(g.attrPatterns) > 0 {
		found := false
		attrs.Range(func(k string, _ pcommon.Value) bool {
			for _, pat := range g.attrPatterns {
				if ok, _ := path.Match(pat, k); ok {
					found = true
					return false // stop iteration
				}
			}
			return true
		})
		if found {
			return true
		}
	}

	if len(g.resPatterns) > 0 {
		found := false
		resourceAttrs.Range(func(k string, _ pcommon.Value) bool {
			for _, pat := range g.resPatterns {
				if ok, _ := path.Match(pat, k); ok {
					found = true
					return false // stop iteration
				}
			}
			return true
		})
		if found {
			return true
		}
	}

	return false
}

// applyRule finds the first existing source and writes its value to the target.
// If move is true the source key is deleted after copying.
// The target is written to resource attributes when writeToResource is true,
// otherwise to span/log attributes.
func applyRule(rule *parsedRule, attrs, resourceAttrs pcommon.Map) {
	for i := range rule.sources {
		src := &rule.sources[i]

		var (
			val pcommon.Value
			ok  bool
		)
		if src.isResource {
			val, ok = resourceAttrs.Get(src.bare)
		} else {
			val, ok = attrs.Get(src.bare)
		}
		if !ok {
			continue
		}

		// Write to the target context.
		var dest pcommon.Value
		if rule.writeToResource {
			dest = resourceAttrs.PutEmpty(rule.target)
		} else {
			dest = attrs.PutEmpty(rule.target)
		}
		val.CopyTo(dest)

		if rule.move {
			if src.isResource {
				resourceAttrs.Remove(src.bare)
			} else {
				attrs.Remove(src.bare)
			}
		}
		return // first match wins
	}
}
