package bodyparser

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/bytedance/sonic"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type JSON struct {
	sonic.Config
}

type JSONLog struct {
	Timestamp      int64          `json:"timestamp"`
	TraceID        string         `json:"trace_id"`
	SpanID         string         `json:"span_id"`
	TraceFlags     int            `json:"trace_flags"`
	SeverityText   string         `json:"severity_text"`
	SeverityNumber int            `json:"severity_number"`
	Attributes     map[string]any `json:"attributes"`
	Resources      map[string]any `json:"resources"`
	Body           any            `json:"body"`
}

func NewJsonBodyParser() *JSON {
	return &JSON{
		Config: sonic.Config{UseInt64: true},
	}
}

func (l *JSON) Parse(body []byte) (plog.Logs, int, error) {
	data := []map[string]any{}
	dec := l.Config.Froze().NewDecoder(bytes.NewReader(body))
	err := dec.Decode(&data)
	if err != nil {
		return plog.NewLogs(), 0, fmt.Errorf("error unmarshalling data:%w", err)
	}

	jsonLogArray := []JSONLog{}
	for _, log := range data {
		jsonLog := JSONLog{}
		for key, val := range log {
			switch key {
			case "timestamp":
				// nanosecond epoch
				switch v := val.(type) {
				case float64:
					jsonLog.Timestamp = getEpochNano(int64(v))
				case int64:
					jsonLog.Timestamp = getEpochNano(v)
				default:
					return plog.NewLogs(), 0, fmt.Errorf("timestamp must be a uint64 nanoseconds since Unix epoch")
				}
			case "trace_id":
				data, ok := val.(string)
				if !ok {
					return plog.NewLogs(), 0, fmt.Errorf("trace_id must be a hex string")
				}
				jsonLog.TraceID = data
			case "span_id":
				data, ok := val.(string)
				if !ok {
					return plog.NewLogs(), 0, fmt.Errorf("span_id must be a hex string")
				}
				jsonLog.SpanID = data
			case "trace_flags":
				switch v := val.(type) {
				case float64:
					jsonLog.TraceFlags = int(v)
				case int64:
					jsonLog.TraceFlags = int(v)
				default:
					return plog.NewLogs(), 0, fmt.Errorf("%s must be a number", key)
				}
			case "severity_text":
				data, ok := val.(string)
				if !ok {
					return plog.NewLogs(), 0, fmt.Errorf("severity_text must be a string")
				}
				jsonLog.SeverityText = data
			case "severity_number":
				switch v := val.(type) {
				case float64:
					jsonLog.SeverityNumber = int(v)
				case int64:
					jsonLog.SeverityNumber = int(v)
				default:
					return plog.NewLogs(), 0, fmt.Errorf("%s must be a number", key)
				}
			case "attributes":
				data, ok := val.(map[string]interface{})
				if !ok {
					return plog.NewLogs(), 0, fmt.Errorf("attributes must be a map")
				}
				jsonLog.Attributes = data
			case "resources":
				data, ok := val.(map[string]interface{})
				if !ok {
					return plog.NewLogs(), 0, fmt.Errorf("resources must be a map")
				}
				jsonLog.Resources = data
			case "message", "body":
				switch v := val.(type) {
				case string:
					jsonLog.Body = v
				case map[string]interface{}:
					jsonLog.Body = v
				default:
					return plog.NewLogs(), 0, fmt.Errorf("%s must be a string or map", key)
				}
			default:
				// if there is any other key present convert it to an attribute
				if jsonLog.Attributes == nil {
					jsonLog.Attributes = map[string]interface{}{}
				}
				jsonLog.Attributes[key] = val
			}
		}
		jsonLogArray = append(jsonLogArray, jsonLog)
	}

	ld := plog.NewLogs()
	for _, log := range jsonLogArray {
		rl := ld.ResourceLogs().AppendEmpty()
		if err := rl.Resource().Attributes().FromRaw(log.Resources); err != nil {
			return plog.NewLogs(), 0, fmt.Errorf("error setting resources: %w", err)
		}
		sl := rl.ScopeLogs().AppendEmpty()
		rec := sl.LogRecords().AppendEmpty()
		if err := rec.Attributes().FromRaw(log.Attributes); err != nil {
			return plog.NewLogs(), 0, fmt.Errorf("error setting attributes: %w", err)
		}
		if err := rec.Body().FromRaw(log.Body); err != nil {
			return plog.NewLogs(), 0, fmt.Errorf("error setting body: %w", err)
		}
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
		rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, log.Timestamp)))
		rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now().UTC()))
		rec.SetSeverityText(log.SeverityText)
		rec.SetSeverityNumber(plog.SeverityNumber(log.SeverityNumber))
		rec.SetFlags(plog.LogRecordFlags(log.TraceFlags))
	}
	return ld, len(data), nil
}

// getEpochNano returns epoch in  nanoseconds
func getEpochNano(epoch int64) int64 {
	epochCopy := epoch
	count := 0
	if epoch == 0 {
		count = 1
	} else {
		for epoch != 0 {
			epoch /= 10
			count++
		}
	}
	return epochCopy * int64(math.Pow(10, float64(19-count)))
}
