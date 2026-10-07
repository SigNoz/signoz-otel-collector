package fieldvalues

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/constants"
)

func (in *recordsInput) addLogs(ld plog.Logs, body *BodyJSONLimits) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		res := len(in.resources)
		lo := len(in.pairs)
		in.pairs = appendAttrPairs(in.pairs, contextResource, rl.Resource().Attributes(), "")
		in.resources = append(in.resources, pairRange{lo: lo, hi: len(in.pairs)})
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			scope := pairRange{lo: len(in.pairs)}
			in.pairs = appendLogScopePairs(in.pairs, sl)
			scope.hi = len(in.pairs)
			lrs := sl.LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				lo := len(in.pairs)
				in.pairs = append(in.pairs, in.pairs[scope.lo:scope.hi]...)
				if lr.SeverityText() != "" {
					in.pairs = append(in.pairs, stringPair(contextLog, "severity_text", lr.SeverityText()))
				}
				if lr.SeverityNumber() != plog.SeverityNumberUnspecified {
					in.pairs = append(in.pairs, numberPair(contextLog, "severity_number", float64(lr.SeverityNumber())))
				}
				in.pairs = appendAttrPairs(in.pairs, contextAttribute, lr.Attributes(), constants.OriginalBodyAttributeKey)
				if body != nil {
					in.pairs = bodyPairs(in.pairs, lr.Body(), *body)
				}
				in.records = append(in.records, preparedRecord{
					resource: res,
					ts:       lr.Timestamp(),
					fallback: lr.ObservedTimestamp(),
					lo:       lo,
					mid:      len(in.pairs),
					hi:       len(in.pairs),
				})
			}
		}
	}
}

func appendLogScopePairs(dst []pair, sl plog.ScopeLogs) []pair {
	if name := sl.Scope().Name(); name != "" {
		dst = append(dst, stringPair(contextScope, "scope_name", name))
	}
	if version := sl.Scope().Version(); version != "" {
		dst = append(dst, stringPair(contextScope, "scope_version", version))
	}
	return appendAttrPairs(dst, contextScope, sl.Scope().Attributes(), "")
}
