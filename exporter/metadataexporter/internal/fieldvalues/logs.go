package fieldvalues

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/constants"
)

func (b *batch) addLogs(ld plog.Logs) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		var res *resourceRef
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			scopePairs := logScopePairs(sl)
			lrs := sl.LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				seen := b.seenMillis(lr.Timestamp(), lr.ObservedTimestamp())
				if res == nil {
					r := b.resource(rl.Resource().Attributes(), seen)
					res = &r
				}
				pairs := make([]pair, 0, len(scopePairs)+lr.Attributes().Len()+2)
				pairs = append(pairs, scopePairs...)
				if lr.SeverityText() != "" {
					pairs = append(pairs, stringPair(contextLog, "severity_text", lr.SeverityText()))
				}
				if lr.SeverityNumber() != plog.SeverityNumberUnspecified {
					pairs = append(pairs, numberPair(contextLog, "severity_number", float64(lr.SeverityNumber())))
				}
				attrs := lr.Attributes()
				if _, ok := attrs.Get(constants.OriginalBodyAttributeKey); ok {
					filtered := plog.NewLogRecord().Attributes()
					attrs.CopyTo(filtered)
					filtered.Remove(constants.OriginalBodyAttributeKey)
					attrs = filtered
				}
				pairs = appendAttrPairs(pairs, contextAttribute, attrs)
				if b.bodyLimits != nil {
					pairs = bodyPairs(pairs, lr.Body(), *b.bodyLimits)
				}
				b.record(*res, pairs, nil, seen)
			}
		}
	}
}

func logScopePairs(sl plog.ScopeLogs) []pair {
	var pairs []pair
	if name := sl.Scope().Name(); name != "" {
		pairs = append(pairs, stringPair(contextScope, "scope_name", name))
	}
	if version := sl.Scope().Version(); version != "" {
		pairs = append(pairs, stringPair(contextScope, "scope_version", version))
	}
	return appendAttrPairs(pairs, contextScope, sl.Scope().Attributes())
}
