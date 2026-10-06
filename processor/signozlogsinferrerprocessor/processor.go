package signozlogsinferrerprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type inferrerProcessor struct {
	names     fieldNames
	telemetry *telemetry
}

type scopeKey struct {
	name, version string
}

func newInferrerProcessor(set component.TelemetrySettings, cfg *Config) (*inferrerProcessor, error) {
	names := newFieldNames(cfg)
	t, err := newTelemetry(set, names)
	if err != nil {
		return nil, err
	}
	return &inferrerProcessor{names: names, telemetry: t}, nil
}

func (p *inferrerProcessor) ProcessLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	st := batchStats{inferences: make([]int64, len(p.telemetry.inferences))}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		p.processResource(rls.At(i), &st)
	}
	p.telemetry.add(&st)
	return ld, nil
}

func (p *inferrerProcessor) processResource(rl plog.ResourceLogs, st *batchStats) {
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

func (p *inferrerProcessor) processScope(sls plog.ScopeLogsSlice, sl plog.ScopeLogs, resource pcommon.Map, st *batchStats) (emptied bool) {
	scope := sl.Scope()
	current := scopeKey{name: scope.Name(), version: scope.Version()}
	inferName, inferVersion := current.name == "", current.version == ""

	lrs := sl.LogRecords()
	var regrouped map[scopeKey]plog.ScopeLogs
	var moved []bool

	for k := 0; k < lrs.Len(); k++ {
		lr := lrs.At(k)
		key := p.inferRecord(lr, scope, resource, current, inferName, inferVersion, st)
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

func (p *inferrerProcessor) inferRecord(
	lr plog.LogRecord,
	scope pcommon.InstrumentationScope,
	resource pcommon.Map,
	current scopeKey,
	inferName, inferVersion bool,
	st *batchStats,
) scopeKey {
	st.records++

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
	if !want.any() {
		return current
	}

	var containers [4]container
	n := 0
	if body := lr.Body(); body.Type() == pcommon.ValueTypeMap {
		containers[n] = container{fields: body.Map(), source: sourceBody}
		n++
	}
	containers[n] = container{fields: lr.Attributes(), source: sourceAttributes}
	containers[n+1] = container{fields: scope.Attributes(), source: sourceScope}
	containers[n+2] = container{fields: resource, source: sourceResource}
	n += 3

	results := p.names.scan(containers[:n], want)
	for t, found := range results.found {
		if found {
			st.inferences[p.telemetry.readSeries[t][results.rank[t]][results.source[t]]]++
		}
	}

	for t, derived := range setSeverity(lr, results, p.names) {
		if derived {
			st.inferences[p.telemetry.derivedSeries[t]]++
		}
	}
	setTraceContext(lr, results)

	key := current
	if results.found[targetScopeName] {
		key.name = results.values[targetScopeName].text
	}
	if results.found[targetScopeVersion] {
		key.version = results.values[targetScopeVersion].text
	}
	return key
}
