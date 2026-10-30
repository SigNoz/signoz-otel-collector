package signozlogsnormalizerprocessor

import (
	"slices"
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

var defaultFields = [targetCount][]string{
	targetSeverityNumber: {"severity_number", "severitynumber"},
	targetSeverityText:   {"severity_text", "severitytext", "severity", "level", "log.level", "log_level", "loglevel", "levelname", "lvl"},
	targetTraceID:        {"trace_id", "traceid", "trace.id"},
	targetSpanID:         {"span_id", "spanid", "span.id"},
	targetScopeName:      {"scope.name", "scope_name", "scopename"},
	targetScopeVersion:   {"scope.version", "scope_version", "scopeversion"},
}

type source int

const (
	sourceBody source = iota
	sourceAttributes
	sourceDerived
	sourceCount
)

var sourceNames = [sourceCount]string{
	sourceBody:       "body",
	sourceAttributes: "attributes",
	sourceDerived:    "derived",
}

// One place to look for a configured field, and the label the metric reports for it.
type lookup struct {
	entries pcommon.Map
	from    source
}

type inferred struct {
	severity plog.SeverityNumber
	text     string
	traceID  pcommon.TraceID
	spanID   pcommon.SpanID
}

type parseFunc func(pcommon.Value) (inferred, bool)

var parsers = [targetCount]parseFunc{
	targetSeverityNumber: parseSeverityNumber,
	targetSeverityText:   parseSeverityText,
	targetTraceID:        parseTraceID,
	targetSpanID:         parseSpanID,
	targetScopeName:      parseNonEmptyString,
	targetScopeVersion:   parseNonEmptyString,
}

type wanted [targetCount]parseFunc

func (w wanted) any() bool {
	for _, parse := range w {
		if parse != nil {
			return true
		}
	}
	return false
}

type fieldMatch struct {
	target target
	rank   int
}

// Configured field names, indexed for lookup by the key found in a log. A dotted name
// matches either a flat key spelled that way or the same path through nested maps, so
// prefixes holds every parent a scan may descend into.
type fieldIndex struct {
	byField  map[string][]fieldMatch
	prefixes map[string]struct{}
	byTarget [targetCount][]string
	move     [targetCount]bool
}

func newFieldIndex(c *Config) fieldIndex {
	f := fieldIndex{byField: map[string][]fieldMatch{}, prefixes: map[string]struct{}{}}
	for t, field := range c.fields() {
		f.add(target(t), *field)
	}
	return f
}

func (f *fieldIndex) add(t target, c FieldConfig) {
	if !c.Enabled {
		return
	}
	f.move[t] = c.Action == ActionMove
	fields := c.Fields
	if len(fields) == 0 {
		fields = defaultFields[t]
	}
	for _, name := range fields {
		field := normalizeField(name)
		if field == "" || slices.Contains(f.byTarget[t], field) {
			continue
		}
		f.byField[field] = append(f.byField[field], fieldMatch{target: t, rank: len(f.byTarget[t])})
		f.byTarget[t] = append(f.byTarget[t], field)
		for i, r := range field {
			if r == '.' {
				f.prefixes[field[:i]] = struct{}{}
			}
		}
	}
}

func (f *fieldIndex) tracks(t target) bool {
	return len(f.byTarget[t]) > 0
}

func (f *fieldIndex) empty() bool {
	return len(f.byField) == 0
}

func normalizeField(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type scanResult struct {
	value   inferred
	found   bool
	rank    int
	source  source
	rawPath string
	key     string
	in      pcommon.Map
}

type scanResults [targetCount]scanResult

func (f *fieldIndex) scan(lookups []lookup, want wanted, results *scanResults) {
	for _, l := range lookups {
		f.scanMap(l.entries, "", "", l.from, want, results)
	}
}

// path is lowercased to match the index; rawPath keeps the log's own spelling, which
// is what breaks a tie between two keys matching the same configured field.
func (f *fieldIndex) scanMap(m pcommon.Map, prefix, rawPrefix string, from source, want wanted, results *scanResults) {
	m.Range(func(key string, value pcommon.Value) bool {
		path, rawPath := prefix+normalizeField(key), rawPrefix+key
		for _, match := range f.byField[path] {
			parse := want[match.target]
			if parse == nil || !results[match.target].losesTo(match.rank, from, rawPath) {
				continue
			}
			parsed, ok := parse(value)
			if !ok {
				continue
			}
			results[match.target] = scanResult{
				value: parsed, found: true, rank: match.rank, source: from,
				rawPath: rawPath, key: key, in: m,
			}
		}
		if _, descend := f.prefixes[path]; descend && value.Type() == pcommon.ValueTypeMap {
			f.scanMap(value.Map(), path+".", rawPath+".", from, want, results)
		}
		return true
	})
}

// The configured order of a field decides. Where it was found, then the log's own
// spelling of it, only break a tie between two keys matching the same configured field.
func (r scanResult) losesTo(rank int, src source, rawPath string) bool {
	switch {
	case !r.found:
		return true
	case rank != r.rank:
		return rank < r.rank
	case src != r.source:
		return src < r.source
	default:
		return rawPath < r.rawPath
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
