package signozspanmapperprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozspanmapperprocessor"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type parsedKey struct {
	bare       string // key without any prefix
	isResource bool   // true → read from resource attributes
	move       bool   // true → delete the source after copying to the target
}

type parsedRule struct {
	target          string
	writeToResource bool // context == ContextResource
	sources         []parsedKey
}

type parsedGroup struct {
	// Substrings split by where they are matched. A group's condition is met
	// when any attribute or resource key contains one of these substrings.
	attrPatterns []string // substrings matched against span/log attribute keys
	resPatterns  []string // substrings matched against resource attribute keys
	rules        []parsedRule
}

type spanMappingProcessor struct {
	groups []parsedGroup
}

// newProcessor pre-parses the configuration so that the hot path (per span/log)
// performs no string allocation or parsing. The per-source action is resolved
// to a single bool (`move`) — an empty action means ActionCopy.
func newProcessor(cfg *Config) *spanMappingProcessor {
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
			}
			for k, src := range rule.Sources {
				bare, isRes := isResourceKey(src.Key)
				pr.sources[k] = parsedKey{
					bare:       bare,
					isResource: isRes,
					move:       src.Action == ActionMove,
				}
			}
			pg.rules[j] = pr
		}
		groups[i] = pg
	}
	return &spanMappingProcessor{groups: groups}
}

// ProcessTraces applies attribute mappings to every span in the batch.
func (p *spanMappingProcessor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	resMatched := make([]bool, len(p.groups))
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resourceAttrs := rs.Resource().Attributes()

		// Resource attributes are invariant across all spans under this
		// ResourceSpans, so evaluate each group's resource condition once here
		// rather than per span.
		for gi := range p.groups {
			resMatched[gi] = matchesAny(resourceAttrs, p.groups[gi].resPatterns)
		}

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.applyGroups(spans.At(k).Attributes(), resourceAttrs, resMatched)
			}
		}
	}
	return td, nil
}

// applyGroups iterates groups and applies the rules of each group whose
// exists_any condition is satisfied. resMatched[i] reports whether group i's
// resource condition was already satisfied for the enclosing resource, so only
// the attribute patterns need to be checked per span.
func (p *spanMappingProcessor) applyGroups(attrs, resourceAttrs pcommon.Map, resMatched []bool) {
	for i := range p.groups {
		g := &p.groups[i]
		if !resMatched[i] && !matchesAny(attrs, g.attrPatterns) {
			continue
		}
		for j := range g.rules {
			applyRule(&g.rules[j], attrs, resourceAttrs)
		}
	}
}

// matchesAny returns true when at least one key in m contains any of the given
// substrings. It iterates only when there are substrings to check and
// short-circuits as soon as a match is found.
func matchesAny(m pcommon.Map, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	found := false
	m.Range(func(k string, _ pcommon.Value) bool {
		for _, pat := range patterns {
			if strings.Contains(k, pat) {
				found = true
				return false // stop iteration
			}
		}
		return true
	})
	return found
}

// applyRule finds the first existing source and writes its value to the target.
// The source-level move flag controls whether the source key is deleted after
// the copy. The target is written to resource attributes when writeToResource
// is true, otherwise to span/log attributes.
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

		if src.move {
			if src.isResource {
				resourceAttrs.Remove(src.bare)
			} else {
				attrs.Remove(src.bare)
			}
		}
		return // first match wins
	}
}
