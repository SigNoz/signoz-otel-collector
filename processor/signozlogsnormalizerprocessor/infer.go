package signozlogsnormalizerprocessor

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type scopeKey struct {
	name, version string
}

func (p *normalizeProcessor) processResource(rl plog.ResourceLogs, st *batchStats) {
	sls := rl.ScopeLogs()
	scopeCount := sls.Len()
	var emptied []bool

	for j := 0; j < scopeCount; j++ {
		if p.processScope(sls, sls.At(j), st) {
			if emptied == nil {
				emptied = make([]bool, scopeCount)
			}
			emptied[j] = true
		}
	}

	if emptied != nil {
		removeMarked(emptied, sls.RemoveIf)
	}
}

// RemoveIf visits in order, so a counter maps each call back to its index. Entries
// appended during the pass sit past the end of marked and are always kept.
func removeMarked[T any](marked []bool, removeIf func(func(T) bool)) {
	i := 0
	removeIf(func(T) bool {
		remove := i < len(marked) && marked[i]
		i++
		return remove
	})
}

func (p *normalizeProcessor) processScope(sls plog.ScopeLogsSlice, sl plog.ScopeLogs, st *batchStats) (emptied bool) {
	scope := sl.Scope()
	current := scopeKey{name: scope.Name(), version: scope.Version()}
	inferName, inferVersion := current.name == "", current.version == ""

	lrs := sl.LogRecords()
	var regrouped map[scopeKey]plog.ScopeLogs
	var moved []bool

	for k := 0; k < lrs.Len(); k++ {
		lr := lrs.At(k)
		if p.bodyEnabled {
			p.normalizeBody(lr, st)
		}
		if p.fields.empty() {
			continue
		}
		key := p.inferRecord(lr, current, inferName, inferVersion, st)
		if key == current {
			continue
		}

		dest, ok := regrouped[key]
		if !ok {
			if regrouped == nil {
				regrouped = map[scopeKey]plog.ScopeLogs{}
				moved = make([]bool, lrs.Len())
			}
			dest = appendScope(sls, sl, key)
			regrouped[key] = dest
		}
		lr.MoveTo(dest.LogRecords().AppendEmpty())
		moved[k] = true
	}

	if moved == nil {
		return false
	}
	removeMarked(moved, lrs.RemoveIf)
	return lrs.Len() == 0
}

// Scope name and version live on the ScopeLogs, so a record that infers either has to
// move to one carrying it rather than rename the scope its siblings share.
func appendScope(sls plog.ScopeLogsSlice, src plog.ScopeLogs, key scopeKey) plog.ScopeLogs {
	dest := sls.AppendEmpty()
	dest.SetSchemaUrl(src.SchemaUrl())
	src.Scope().CopyTo(dest.Scope())
	dest.Scope().SetName(key.name)
	dest.Scope().SetVersion(key.version)
	return dest
}

func (p *normalizeProcessor) lookFor(lr plog.LogRecord, inferName, inferVersion bool) wanted {
	unset := [targetCount]bool{
		targetSeverityNumber: lr.SeverityNumber() == plog.SeverityNumberUnspecified,
		targetSeverityText:   lr.SeverityText() == "",
		targetTraceID:        lr.TraceID().IsEmpty(),
		targetSpanID:         lr.SpanID().IsEmpty(),
		targetScopeName:      inferName,
		targetScopeVersion:   inferVersion,
	}
	var want wanted
	for t := range want {
		if unset[t] && p.fields.tracks(target(t)) {
			want[t] = parsers[t]
		}
	}
	return want
}

func (p *normalizeProcessor) inferRecord(
	lr plog.LogRecord,
	current scopeKey,
	inferName, inferVersion bool,
	st *batchStats,
) scopeKey {
	want := p.lookFor(lr, inferName, inferVersion)
	if !want.any() {
		return current
	}

	var lookups [2]lookup
	n := 0
	if body, ok := p.bodyFields(lr.Body()); ok {
		lookups[n] = lookup{entries: body, from: sourceBody}
		n++
	}
	lookups[n] = lookup{entries: lr.Attributes(), from: sourceAttributes}
	n++

	var results scanResults
	p.fields.scan(lookups[:n], want, &results)
	for t, r := range results {
		if r.found {
			st.inferences[p.telemetry.readSeries[t][r.rank][r.source]]++
		}
	}

	for t, derived := range setSeverity(lr, &results, &p.fields) {
		if derived {
			st.inferences[p.telemetry.derivedSeries[t]]++
		}
	}
	setTraceContext(lr, &results)
	removeMovedFields(&results, &p.fields)

	key := current
	if r := results[targetScopeName]; r.found {
		key.name = r.value.text
	}
	if r := results[targetScopeVersion]; r.found {
		key.version = r.value.text
	}
	return key
}

func (p *normalizeProcessor) bodyFields(body pcommon.Value) (pcommon.Map, bool) {
	switch body.Type() {
	case pcommon.ValueTypeMap:
		return body.Map(), true
	case pcommon.ValueTypeStr:
		if p.parseJSONObject(body.Str(), body) {
			return body.Map(), true
		}
	}
	return pcommon.Map{}, false
}

func removeMovedFields(results *scanResults, fields *fieldIndex) {
	for t, r := range results {
		if r.found && fields.move[t] {
			r.in.Remove(r.key)
		}
	}
}
