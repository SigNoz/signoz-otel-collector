package signozlogsnormalizerprocessor

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type scopeKey struct {
	name, version string
}

type sharedScan struct {
	containers [2]container
	want       wanted
	results    scanResults
	scanned    bool
}

func (s *sharedScan) get(names *fieldNames) *scanResults {
	if !s.scanned {
		names.scan(s.containers[:], s.want, &s.results)
		s.scanned = true
	}
	return &s.results
}

func (p *normalizeProcessor) processResource(rl plog.ResourceLogs, st *batchStats) {
	resource := rl.Resource().Attributes()
	sls := rl.ScopeLogs()
	scopeCount := sls.Len()
	var emptied []bool

	for j := 0; j < scopeCount; j++ {
		if p.processScope(sls, sls.At(j), resource, st) {
			if emptied == nil {
				emptied = make([]bool, scopeCount)
			}
			emptied[j] = true
		}
	}

	if emptied != nil {
		j := 0
		sls.RemoveIf(func(plog.ScopeLogs) bool {
			remove := j < scopeCount && emptied[j]
			j++
			return remove
		})
	}
}

func (p *normalizeProcessor) processScope(sls plog.ScopeLogsSlice, sl plog.ScopeLogs, resource pcommon.Map, st *batchStats) (emptied bool) {
	scope := sl.Scope()
	current := scopeKey{name: scope.Name(), version: scope.Version()}
	inferName, inferVersion := current.name == "", current.version == ""

	shared := sharedScan{
		containers: [2]container{
			{fields: scope.Attributes(), source: sourceScope},
			{fields: resource, source: sourceResource},
		},
		want: p.wanted(plog.NewLogRecord(), inferName, inferVersion),
	}

	lrs := sl.LogRecords()
	var regrouped map[scopeKey]plog.ScopeLogs
	var moved []bool

	for k := 0; k < lrs.Len(); k++ {
		lr := lrs.At(k)
		if p.bodyEnabled {
			p.normalizeBody(lr, st)
		}
		if !p.names.any() {
			continue
		}
		key := p.inferRecord(lr, &shared, current, inferName, inferVersion, st)
		if key == current {
			continue
		}

		dest, ok := regrouped[key]
		if !ok {
			if regrouped == nil {
				regrouped = map[scopeKey]plog.ScopeLogs{}
				moved = make([]bool, lrs.Len())
			}
			dest = sls.AppendEmpty()
			dest.SetSchemaUrl(sl.SchemaUrl())
			scope.CopyTo(dest.Scope())
			dest.Scope().SetName(key.name)
			dest.Scope().SetVersion(key.version)
			regrouped[key] = dest
		}
		lr.MoveTo(dest.LogRecords().AppendEmpty())
		moved[k] = true
	}

	if moved == nil {
		return false
	}
	k := 0
	lrs.RemoveIf(func(plog.LogRecord) bool {
		remove := moved[k]
		k++
		return remove
	})
	return lrs.Len() == 0
}

func (p *normalizeProcessor) wanted(lr plog.LogRecord, inferName, inferVersion bool) wanted {
	var want wanted
	if lr.SeverityNumber() == plog.SeverityNumberUnspecified {
		want[targetSeverityNumber] = parseSeverityNumber
	}
	if lr.SeverityText() == "" {
		want[targetSeverityText] = parseSeverityText
	}
	if lr.TraceID().IsEmpty() {
		want[targetTraceID] = parseTraceID
	}
	if lr.SpanID().IsEmpty() {
		want[targetSpanID] = parseSpanID
	}
	if inferName {
		want[targetScopeName] = parseNonEmptyString
	}
	if inferVersion {
		want[targetScopeVersion] = parseNonEmptyString
	}
	for t := range want {
		if !p.names.enabled(target(t)) {
			want[t] = nil
		}
	}
	return want
}

func (p *normalizeProcessor) inferRecord(
	lr plog.LogRecord,
	shared *sharedScan,
	current scopeKey,
	inferName, inferVersion bool,
	st *batchStats,
) scopeKey {
	want := p.wanted(lr, inferName, inferVersion)
	if !want.any() {
		return current
	}

	var containers [2]container
	n := 0
	if body := lr.Body(); body.Type() == pcommon.ValueTypeMap {
		containers[n] = container{fields: body.Map(), source: sourceBody}
		n++
	}
	containers[n] = container{fields: lr.Attributes(), source: sourceAttributes}
	n++

	var results scanResults
	p.names.scan(containers[:n], want, &results)
	for t := range want {
		if want[t] == nil || results.found[t] {
			continue
		}
		if from := shared.get(&p.names); from.found[t] {
			results.values[t] = from.values[t]
			results.found[t] = true
			results.rank[t] = from.rank[t]
			results.source[t] = from.source[t]
		}
	}
	for t, found := range results.found {
		if found {
			st.inferences[p.telemetry.readSeries[t][results.rank[t]][results.source[t]]]++
		}
	}

	for t, derived := range setSeverity(lr, &results, &p.names) {
		if derived {
			st.inferences[p.telemetry.derivedSeries[t]]++
		}
	}
	setTraceContext(lr, &results)

	key := current
	if results.found[targetScopeName] {
		key.name = results.values[targetScopeName].text
	}
	if results.found[targetScopeVersion] {
		key.version = results.values[targetScopeVersion].text
	}
	return key
}
