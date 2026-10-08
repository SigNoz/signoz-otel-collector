package signozlogsnormalizerprocessor

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type target int

const (
	targetSeverityNumber target = iota
	targetSeverityText
	targetTraceID
	targetSpanID
	targetScopeName
	targetScopeVersion
	targetCount
)

var targetNames = [targetCount]string{
	targetSeverityNumber: "severity_number",
	targetSeverityText:   "severity_text",
	targetTraceID:        "trace_id",
	targetSpanID:         "span_id",
	targetScopeName:      "scope_name",
	targetScopeVersion:   "scope_version",
}

type source int

const (
	sourceBody source = iota
	sourceAttributes
	sourceScope
	sourceResource
	sourceDerived
	sourceCount
)

var sourceNames = [sourceCount]string{
	sourceBody:       "body",
	sourceAttributes: "attributes",
	sourceScope:      "scope",
	sourceResource:   "resource",
	sourceDerived:    "derived",
}

type container struct {
	fields pcommon.Map
	source source
}

type inferred struct {
	severity plog.SeverityNumber
	text     string
	traceID  pcommon.TraceID
	spanID   pcommon.SpanID
}

type parseFunc func(pcommon.Value) (inferred, bool)

type wanted [targetCount]parseFunc

func (w wanted) any() bool {
	for _, parse := range w {
		if parse != nil {
			return true
		}
	}
	return false
}

type nameMatch struct {
	target target
	rank   int
}

type fieldNames struct {
	matches  map[string][]nameMatch
	byTarget [targetCount][]string
}

func newFieldNames(c FieldsConfig) fieldNames {
	f := &fieldNames{matches: map[string][]nameMatch{}}
	if !c.Enabled {
		return *f
	}
	f.add(targetSeverityNumber, c.SeverityNumber)
	f.add(targetSeverityText, c.SeverityText)
	f.add(targetTraceID, c.TraceID)
	f.add(targetSpanID, c.SpanID)
	f.add(targetScopeName, c.ScopeName)
	f.add(targetScopeVersion, c.ScopeVersion)
	return *f
}

func (f *fieldNames) add(t target, names []string) {
	for _, name := range names {
		field := normalizeFieldName(name)
		if field == "" || f.knows(t, field) {
			continue
		}
		f.matches[field] = append(f.matches[field], nameMatch{target: t, rank: len(f.byTarget[t])})
		f.byTarget[t] = append(f.byTarget[t], field)
	}
}

func (f *fieldNames) enabled(t target) bool {
	return len(f.byTarget[t]) > 0
}

func (f *fieldNames) any() bool {
	return len(f.matches) > 0
}

func (f *fieldNames) knows(t target, field string) bool {
	for _, match := range f.matches[field] {
		if match.target == t {
			return true
		}
	}
	return false
}

func normalizeFieldName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type scanResults struct {
	values [targetCount]inferred
	found  [targetCount]bool
	rank   [targetCount]int
	source [targetCount]source
}

func (f *fieldNames) scan(containers []container, want wanted, results *scanResults) {
	var bestRank [targetCount]int
	var bestKey [targetCount]string

	remaining := 0
	for t := range want {
		bestRank[t] = -1
		if want[t] != nil {
			remaining++
		}
	}

	for _, container := range containers {
		if remaining == 0 {
			break
		}

		container.fields.Range(func(key string, value pcommon.Value) bool {
			for _, match := range f.matches[normalizeFieldName(key)] {
				parse := want[match.target]
				if parse == nil {
					continue
				}
				rank, best := bestRank[match.target], bestKey[match.target]
				if rank >= 0 && (match.rank > rank || (match.rank == rank && key >= best)) {
					continue
				}
				parsed, ok := parse(value)
				if !ok {
					continue
				}
				results.values[match.target] = parsed
				results.found[match.target] = true
				results.rank[match.target] = match.rank
				results.source[match.target] = container.source
				bestRank[match.target], bestKey[match.target] = match.rank, key
			}
			return true
		})

		for t := range want {
			if want[t] != nil && results.found[t] {
				want[t] = nil
				remaining--
			}
		}
	}
}

func parseNonEmptyString(value pcommon.Value) (inferred, bool) {
	if value.Type() != pcommon.ValueTypeStr {
		return inferred{}, false
	}
	str := strings.TrimSpace(value.Str())
	if str == "" {
		return inferred{}, false
	}
	return inferred{text: str}, true
}
