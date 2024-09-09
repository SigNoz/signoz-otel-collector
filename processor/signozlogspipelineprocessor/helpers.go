package signozlogspipelineprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// Stanza operator that consumes stanza entries, converts
// them to pdata.Log and and passes them on to the next otel consumer

// Implements stanza operator.Operator
type stanzaToOtelConsumer struct {
	nextConsumer consumer.Logs

	logger *zap.Logger

	lock             sync.Mutex
	processedEntries []*entry.Entry
}

// Operator interface
func (c *stanzaToOtelConsumer) ID() string {
	return "stanza-otel-consumer"
}

// Type returns the type of the operator.
func (c *stanzaToOtelConsumer) Type() string {
	return "stanza-otel-consumer"
}

// Start will start the operator.
func (c *stanzaToOtelConsumer) Start(_ operator.Persister) error {
	return nil
}

// Stop will stop the operator.
func (c *stanzaToOtelConsumer) Stop() error {
	return nil
}

// CanOutput indicates if the operator will output entries to other operators.
func (c *stanzaToOtelConsumer) CanOutput() bool {
	return false
}

// Outputs returns the list of connected outputs.
func (c *stanzaToOtelConsumer) Outputs() []operator.Operator {
	return []operator.Operator{}
}

// GetOutputIDs returns the list of connected outputs.
func (c *stanzaToOtelConsumer) GetOutputIDs() []string {
	return []string{}
}

// SetOutputs will set the connected outputs.
func (c *stanzaToOtelConsumer) SetOutputs([]operator.Operator) error {
	return fmt.Errorf("outputs not supported")
}

// SetOutputIDs will set the connected outputs' IDs.
func (c *stanzaToOtelConsumer) SetOutputIDs([]string) {
}

// CanProcess indicates if the operator will process entries from other operators.
func (c *stanzaToOtelConsumer) CanProcess() bool {
	return true
}

// Process will process an entry from an operator.
func (c *stanzaToOtelConsumer) Process(ctx context.Context, entry *entry.Entry) error {
	// Convert to pdata.Log and pass it on to c.nextConsumer
	// fmt.Println("stanza to otel sink received", entry)

	c.lock.Lock()
	defer c.lock.Unlock()
	c.processedEntries = append(c.processedEntries, entry)
	return nil
}

func (c *stanzaToOtelConsumer) flush(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	plogs := convertEntriesToPlogs(c.processedEntries)

	err := c.nextConsumer.ConsumeLogs(ctx, plogs)
	if err != nil {
		return err
	}

	c.processedEntries = []*entry.Entry{}

	return nil
}

func (c *stanzaToOtelConsumer) Logger() *zap.Logger {
	return c.logger
}

func HashResource(d map[string]any) string {
	// TODO(Raj): Bring in hashing logic from logstransform
	j, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	return string(j)
}

func convertEntriesToPlogs(entries []*entry.Entry) plog.Logs {
	resourceHashToIdx := make(map[string]int)
	scopeIdxByResource := make(map[string]map[string]int)

	pLogs := plog.NewLogs()
	var sl plog.ScopeLogs

	for _, e := range entries {
		resourceID := HashResource(e.Resource)
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
		upsertToAttributeVal(v, dest.PutEmpty(k))
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
		copy(buffer[0:16], ent.TraceID)
		dest.SetTraceID(buffer)
	}
	if ent.SpanID != nil {
		var buffer [8]byte
		copy(buffer[0:8], ent.SpanID)
		dest.SetSpanID(buffer)
	}
	if ent.TraceFlags != nil && len(ent.TraceFlags) > 0 {
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
