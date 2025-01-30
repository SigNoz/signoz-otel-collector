// Brought in as is from pkg/stanza/adapter/converter.go in opentelemetry-collector-contrib
package signozlogspipelineprocessor

import (
	"fmt"
	"strings"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func convertEntriesToPlogs(entries []*entry.Entry) plog.Logs {
	resourceHashToIdx := make(map[uint64]int)
	scopeIdxByResource := make(map[uint64]map[string]int)

	pLogs := plog.NewLogs()
	var sl plog.ScopeLogs

	for _, e := range entries {
		resourceID := adapter.HashResource(e.Resource)
		var rl plog.ResourceLogs

		resourceIdx, ok := resourceHashToIdx[resourceID]
		if !ok {
			resourceHashToIdx[resourceID] = pLogs.ResourceLogs().Len()

			rl = pLogs.ResourceLogs().AppendEmpty()
			upsertToMap(e.Resource, rl.Resource().Attributes())

			scopeIdxByResource[resourceID] = map[string]int{e.ScopeName: 0}
			sl = rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(e.ScopeName)
		} else {
			rl = pLogs.ResourceLogs().At(resourceIdx)
			scopeIdxInResource, ok := scopeIdxByResource[resourceID][e.ScopeName]
			if !ok {
				scopeIdxByResource[resourceID][e.ScopeName] = rl.ScopeLogs().Len()
				sl = rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName(e.ScopeName)
			} else {
				sl = pLogs.ResourceLogs().At(resourceIdx).ScopeLogs().At(scopeIdxInResource)
			}
		}
		convertInto(e, sl.LogRecords().AppendEmpty())
	}

	return pLogs
}

func upsertToMap(obsMap map[string]any, dest pcommon.Map) {
	dest.EnsureCapacity(len(obsMap))
	for k, v := range obsMap {
		if !strings.HasPrefix(k, signozstanzaentry.InternalTempAttributePrefix) {
			upsertToAttributeVal(v, dest.PutEmpty(k))
		}
	}
}

func upsertToAttributeVal(value any, dest pcommon.Value) {
	switch t := value.(type) {
	case bool:
		dest.SetBool(t)
	case string:
		dest.SetStr(t)
	case []string:
		upsertStringsToSlice(t, dest.SetEmptySlice())
	case []byte:
		dest.SetEmptyBytes().FromRaw(t)
	case int64:
		dest.SetInt(t)
	case int32:
		dest.SetInt(int64(t))
	case int16:
		dest.SetInt(int64(t))
	case int8:
		dest.SetInt(int64(t))
	case int:
		dest.SetInt(int64(t))
	case uint64:
		dest.SetInt(int64(t))
	case uint32:
		dest.SetInt(int64(t))
	case uint16:
		dest.SetInt(int64(t))
	case uint8:
		dest.SetInt(int64(t))
	case uint:
		dest.SetInt(int64(t))
	case float64:
		dest.SetDouble(t)
	case float32:
		dest.SetDouble(float64(t))
	case map[string]any:
		upsertToMap(t, dest.SetEmptyMap())
	case []any:
		upsertToSlice(t, dest.SetEmptySlice())
	default:
		dest.SetStr(fmt.Sprintf("%v", t))
	}
}

func upsertToSlice(obsArr []any, dest pcommon.Slice) {
	dest.EnsureCapacity(len(obsArr))
	for _, v := range obsArr {
		upsertToAttributeVal(v, dest.AppendEmpty())
	}
}

func upsertStringsToSlice(obsArr []string, dest pcommon.Slice) {
	dest.EnsureCapacity(len(obsArr))
	for _, v := range obsArr {
		dest.AppendEmpty().SetStr(v)
	}
}

func convertInto(ent *entry.Entry, dest plog.LogRecord) {
	if !ent.Timestamp.IsZero() {
		dest.SetTimestamp(pcommon.NewTimestampFromTime(ent.Timestamp))
	}
	dest.SetObservedTimestamp(pcommon.NewTimestampFromTime(ent.ObservedTimestamp))
	dest.SetSeverityNumber(sevMap[ent.Severity])
	if ent.SeverityText == "" {
		dest.SetSeverityText(defaultSevTextMap[ent.Severity])
	} else {
		dest.SetSeverityText(ent.SeverityText)
	}

	upsertToMap(ent.Attributes, dest.Attributes())

	if ent.Body != nil {
		upsertToAttributeVal(ent.Body, dest.Body())
	}

	if ent.TraceID != nil {
		var buffer [16]byte
		// ensure buffer gets padded to left if len(ent.TraceID) != 16
		copyStartIdx := (max(0, 16-len(ent.TraceID)))
		copy(buffer[copyStartIdx:16], ent.TraceID)
		dest.SetTraceID(buffer)
	}
	if ent.SpanID != nil {
		var buffer [8]byte
		copyStartIdx := (max(0, 8-len(ent.SpanID)))
		copy(buffer[copyStartIdx:8], ent.SpanID)
		dest.SetSpanID(buffer)
	}
	if len(ent.TraceFlags) > 0 {
		// The 8 least significant bits are the trace flags as defined in W3C Trace
		// Context specification. Don't override the 24 reserved bits.
		flags := uint32(ent.TraceFlags[0])
		dest.SetFlags(plog.LogRecordFlags(flags))
	}
}

var sevMap = map[entry.Severity]plog.SeverityNumber{
	entry.Default: plog.SeverityNumberUnspecified,
	entry.Trace:   plog.SeverityNumberTrace,
	entry.Trace2:  plog.SeverityNumberTrace2,
	entry.Trace3:  plog.SeverityNumberTrace3,
	entry.Trace4:  plog.SeverityNumberTrace4,
	entry.Debug:   plog.SeverityNumberDebug,
	entry.Debug2:  plog.SeverityNumberDebug2,
	entry.Debug3:  plog.SeverityNumberDebug3,
	entry.Debug4:  plog.SeverityNumberDebug4,
	entry.Info:    plog.SeverityNumberInfo,
	entry.Info2:   plog.SeverityNumberInfo2,
	entry.Info3:   plog.SeverityNumberInfo3,
	entry.Info4:   plog.SeverityNumberInfo4,
	entry.Warn:    plog.SeverityNumberWarn,
	entry.Warn2:   plog.SeverityNumberWarn2,
	entry.Warn3:   plog.SeverityNumberWarn3,
	entry.Warn4:   plog.SeverityNumberWarn4,
	entry.Error:   plog.SeverityNumberError,
	entry.Error2:  plog.SeverityNumberError2,
	entry.Error3:  plog.SeverityNumberError3,
	entry.Error4:  plog.SeverityNumberError4,
	entry.Fatal:   plog.SeverityNumberFatal,
	entry.Fatal2:  plog.SeverityNumberFatal2,
	entry.Fatal3:  plog.SeverityNumberFatal3,
	entry.Fatal4:  plog.SeverityNumberFatal4,
}

var defaultSevTextMap = map[entry.Severity]string{
	entry.Default: "",
	entry.Trace:   "TRACE",
	entry.Trace2:  "TRACE2",
	entry.Trace3:  "TRACE3",
	entry.Trace4:  "TRACE4",
	entry.Debug:   "DEBUG",
	entry.Debug2:  "DEBUG2",
	entry.Debug3:  "DEBUG3",
	entry.Debug4:  "DEBUG4",
	entry.Info:    "INFO",
	entry.Info2:   "INFO2",
	entry.Info3:   "INFO3",
	entry.Info4:   "INFO4",
	entry.Warn:    "WARN",
	entry.Warn2:   "WARN2",
	entry.Warn3:   "WARN3",
	entry.Warn4:   "WARN4",
	entry.Error:   "ERROR",
	entry.Error2:  "ERROR2",
	entry.Error3:  "ERROR3",
	entry.Error4:  "ERROR4",
	entry.Fatal:   "FATAL",
	entry.Fatal2:  "FATAL2",
	entry.Fatal3:  "FATAL3",
	entry.Fatal4:  "FATAL4",
}
