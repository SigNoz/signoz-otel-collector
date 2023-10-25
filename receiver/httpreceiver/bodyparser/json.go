package bodyparser

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type JSON struct {
}

type JSONLog struct {
	TraceID        string                 `json:"trace_id"`
	SpanID         string                 `json:"span_id"`
	TraceFlags     int                    `json:"trace_flags"`
	SeverityText   string                 `json:"severity_text"`
	SeverityNumber int                    `json:"severity_number"`
	Attributes     map[string]interface{} `json:"attributes"`
	Resources      map[string]interface{} `json:"resources"`
	Body           string                 `json:"body"`
}

func NewJsonBodyParser() *JSON {
	return &JSON{}
}

func (l *JSON) Parse(body []byte) (plog.Logs, int, error) {

	data := []JSONLog{}
	err := json.Unmarshal(body, &data)
	if err != nil {
		return plog.NewLogs(), 0, err
	}

	ld := plog.NewLogs()
	for _, log := range data {
		rl := ld.ResourceLogs().AppendEmpty()
		rAttrLen := len(log.Resources)
		rl.Resource().Attributes().EnsureCapacity(rAttrLen)
		for k, v := range log.Resources {
			l.AddAttribute(rl.Resource().Attributes(), k, v)
		}
		sl := rl.ScopeLogs().AppendEmpty()
		rec := sl.LogRecords().AppendEmpty()
		attrLen := len(log.Attributes)
		rec.Attributes().EnsureCapacity(attrLen)
		for k, v := range log.Attributes {
			l.AddAttribute(rec.Attributes(), k, v)
		}
		rec.Body().SetStr(log.Body)
		if log.TraceID != "" {
			traceIdByte, err := hex.DecodeString(log.TraceID)
			if err != nil {
				return plog.Logs{}, 0, fmt.Errorf("error decoding trace_id:%w", err)
			}
			var traceID [16]byte
			copy(traceID[:], traceIdByte)
			rec.SetTraceID(pcommon.TraceID(pcommon.TraceID(traceID)))
		}
		if log.SpanID != "" {
			spanIdByte, err := hex.DecodeString(log.SpanID)
			if err != nil {
				return plog.Logs{}, 0, fmt.Errorf("error decoding span_id:%w", err)
			}
			var spanID [8]byte
			copy(spanID[:], spanIdByte)
			rec.SetSpanID(pcommon.SpanID(spanID))
		}
		rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now().UTC()))
		rec.SetSeverityText(log.SeverityText)
		rec.SetSeverityNumber(plog.SeverityNumber(log.SeverityNumber))
		rec.SetFlags(plog.LogRecordFlags(log.TraceFlags))
	}
	return ld, len(data), nil
}

func (l *JSON) AddAttribute(attrs pcommon.Map, key string, value interface{}) {
	switch value.(type) {
	case string:
		attrs.PutStr(key, value.(string))
	case int, int8, int16, int32, int64:
		attrs.PutInt(key, value.(int64))
	case uint, uint8, uint16, uint32, uint64:
		attrs.PutInt(key, int64(value.(uint64)))
	case float32, float64:
		attrs.PutDouble(key, value.(float64))
	case bool:
		attrs.PutBool(key, value.(bool))
	default:
		// ignoring the error for now
		bytes, _ := json.Marshal(value)
		attrs.PutStr(key, string(bytes))
	}

}
