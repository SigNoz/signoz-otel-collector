package bodyparser

import (
	"encoding/hex"
	"encoding/json"
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

func NewJSON() *JSON {
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
			l.AddResourceAttribute(rl, k, v)
		}
		sl := rl.ScopeLogs().AppendEmpty()
		rec := sl.LogRecords().AppendEmpty()
		attrLen := len(log.Attributes)
		rec.Attributes().EnsureCapacity(attrLen)
		for k, v := range log.Attributes {
			l.AddAttribute(rec, k, v)
		}
		rec.Body().SetStr(log.Body)
		if log.TraceID != "" {
			traceIdByte, err := hex.DecodeString(log.TraceID)
			if err != nil {
				return plog.Logs{}, 0, err
			}
			var traceID [16]byte
			copy(traceID[:], traceIdByte)
			rec.SetTraceID(pcommon.TraceID(pcommon.TraceID(traceID)))
		}
		if log.SpanID != "" {
			spanIdByte, err := hex.DecodeString(log.SpanID)
			if err != nil {
				return plog.Logs{}, 0, err
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

func (l *JSON) AddAttribute(log plog.LogRecord, key string, value interface{}) {
	switch value.(type) {
	case string:
		log.Attributes().PutStr(key, value.(string))
	case int, int8, int16, int32, int64:
		log.Attributes().PutInt(key, value.(int64))
	case uint, uint8, uint16, uint32, uint64:
		log.Attributes().PutInt(key, int64(value.(uint64)))
	case float32, float64:
		log.Attributes().PutDouble(key, value.(float64))
	case bool:
		log.Attributes().PutBool(key, value.(bool))
	default:
		// ignoring the error for now
		bytes, _ := json.Marshal(value)
		log.Attributes().PutStr(key, string(bytes))
	}

}

func (l *JSON) AddResourceAttribute(log plog.ResourceLogs, key string, value interface{}) {
	switch value.(type) {
	case string:
		log.Resource().Attributes().PutStr(key, value.(string))
	case int, int8, int16, int32, int64:
		log.Resource().Attributes().PutInt(key, value.(int64))
	case uint, uint8, uint16, uint32, uint64:
		log.Resource().Attributes().PutInt(key, int64(value.(uint64)))
	case float32, float64:
		log.Resource().Attributes().PutDouble(key, value.(float64))
	case bool:
		log.Resource().Attributes().PutBool(key, value.(bool))
	default:
		// ignoring the error for now
		bytes, _ := json.Marshal(value)
		log.Resource().Attributes().PutStr(key, string(bytes))
	}

}
