package json

import (
	"strings"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type fieldTarget int

const (
	targetMessage fieldTarget = iota
	targetSeverityNumber
	targetSeverityText
	targetTraceID
	targetSpanID
	targetTraceFlags
	targetScopeName
	targetScopeVersion
	targetCount
)

// parseFunc reports whether a value can serve as a target, and returns it in the shape the
// target is recorded in. Targets are looked for in one pass, so the parsed value is carried
// as an any and asserted back by whoever asked for that target.
type parseFunc func(value any) (any, bool)

// wantedFields holds a parseFunc for each target still worth looking for on this record.
type wantedFields [targetCount]parseFunc

type nameMatch struct {
	target fieldTarget
	rank   int
}

type fieldConfig struct {
	names map[string][]nameMatch
	move  [targetCount]bool
}

func newFieldConfig(c Config) fieldConfig {
	f := fieldConfig{names: map[string][]nameMatch{}}

	f.addNames(targetMessage, c.MessageFields)
	f.addNames(targetSeverityNumber, c.SeverityNumberFields)
	f.addNames(targetSeverityText, c.SeverityTextFields)
	f.addNames(targetTraceID, c.TraceIDFields)
	f.addNames(targetSpanID, c.SpanIDFields)
	f.addNames(targetTraceFlags, c.TraceFlagsFields)
	f.addNames(targetScopeName, c.ScopeNameFields)
	f.addNames(targetScopeVersion, c.ScopeVersionFields)

	f.move = [targetCount]bool{
		targetSeverityNumber: c.MoveAllFields || c.MoveSeverityNumberField,
		targetSeverityText:   c.MoveAllFields || c.MoveSeverityTextField,
		targetTraceID:        c.MoveAllFields || c.MoveTraceIDField,
		targetSpanID:         c.MoveAllFields || c.MoveSpanIDField,
		targetTraceFlags:     c.MoveAllFields || c.MoveTraceFlagsField,
		targetScopeName:      c.MoveAllFields || c.MoveScopeNameField,
		targetScopeVersion:   c.MoveAllFields || c.MoveScopeVersionField,
	}

	return f
}

func (f *fieldConfig) addNames(target fieldTarget, names []string) {
	rank := 0
	for _, name := range names {
		field := normalizeFieldName(name)
		if field == "" || f.knows(target, field) {
			continue
		}
		f.names[field] = append(f.names[field], nameMatch{target: target, rank: rank})
		rank++
	}
}

func (f fieldConfig) knows(target fieldTarget, field string) bool {
	for _, match := range f.names[field] {
		if match.target == target {
			return true
		}
	}
	return false
}

func normalizeFieldName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func searchOrder(ent *entry.Entry) [4]map[string]any {
	body, _ := ent.Body.(map[string]any)
	return [4]map[string]any{body, ent.Attributes, scopeAttributes(ent), ent.Resource}
}

func scopeAttributes(ent *entry.Entry) map[string]any {
	attributes, _ := ent.Attributes[signozstanzaentry.InternalTempScopeAttributesAttribute].(map[string]any)
	return attributes
}

type fieldSource struct {
	container map[string]any
	key       string
}

func (s fieldSource) remove() {
	delete(s.container, s.key)
}

type scanResult struct {
	value  any
	source fieldSource
	found  bool
}

type scanResults [targetCount]scanResult

// take applies the move setting for a target that was found, and reports the value so the
// caller can record it.
func (f fieldConfig) take(results scanResults, target fieldTarget) (any, bool) {
	result := results[target]
	if !result.found {
		return nil, false
	}
	if f.move[target] {
		result.source.remove()
	}
	return result.value, true
}

// scan walks each container once, looking every key up against the names of all wanted
// targets at once, rather than walking the containers again for each target. A container is
// searched only while some target is still unresolved, and a target is resolved by the first
// container that yields a usable value for it. Within a container the lowest ranked name
// wins, ties going to the lexicographically smaller key so the result never depends on map
// order, and a value the target can't use is passed over in favour of the next candidate.
func (f fieldConfig) scan(containers [4]map[string]any, wanted wantedFields) scanResults {
	var results scanResults
	var bestRank [targetCount]int
	var bestKey [targetCount]string

	remaining := 0
	for target := range wanted {
		bestRank[target] = -1
		if wanted[target] != nil {
			remaining++
		}
	}

	for _, container := range containers {
		if remaining == 0 {
			break
		}

		for key, value := range container {
			for _, match := range f.names[strings.ToLower(key)] {
				parse := wanted[match.target]
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
				results[match.target] = scanResult{
					value:  parsed,
					source: fieldSource{container: container, key: key},
					found:  true,
				}
				bestRank[match.target], bestKey[match.target] = match.rank, key
			}
		}

		for target := range wanted {
			if wanted[target] != nil && bestRank[target] >= 0 {
				wanted[target] = nil
				remaining--
			}
		}
	}

	return results
}

// infer fills in the top level fields the log carries inside itself. Every target is looked
// for in the same pass, so a record costs one walk of its containers rather than one per
// target, and what each container holds is read before any of it is moved out.
func (f fieldConfig) infer(ent *entry.Entry) {
	var wanted wantedFields

	if ent.Severity == entry.Default {
		wanted[targetSeverityNumber] = parseSeverityNumber
	}
	if ent.SeverityText == "" {
		wanted[targetSeverityText] = parseNonEmptyString
	}
	if len(ent.TraceID) == 0 {
		wanted[targetTraceID] = parseTraceID
	}
	if len(ent.SpanID) == 0 {
		wanted[targetSpanID] = parseSpanID
	}
	if len(ent.TraceFlags) == 0 {
		wanted[targetTraceFlags] = parseTraceFlags
	}
	if ent.ScopeName == "" {
		wanted[targetScopeName] = parseNonEmptyString
	}
	if _, exists := ent.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute]; !exists {
		wanted[targetScopeVersion] = parseNonEmptyString
	}

	results := f.scan(searchOrder(ent), wanted)

	f.setSeverity(ent, results)
	f.setTraceContext(ent, results)
	f.setScope(ent, results)
}

func anyValue(value any) (any, bool) {
	return value, value != nil
}

func parseNonEmptyString(value any) (any, bool) {
	str, ok := value.(string)
	if !ok {
		return nil, false
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return nil, false
	}
	return str, true
}
