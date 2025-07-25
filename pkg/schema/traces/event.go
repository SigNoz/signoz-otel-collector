package traces

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zapcore"
)

// This has been copied from the exporter. The exporter needs to read from here.
type Event struct {
	Name         string            `json:"name,omitempty"`
	TimeUnixNano uint64            `json:"timeUnixNano,omitempty"`
	AttributeMap map[string]string `json:"attributeMap,omitempty"`
	IsError      bool              `json:"isError,omitempty"`
}

func (e *Event) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", e.Name)
	enc.AddUint64("timeUnixNano", e.TimeUnixNano)
	enc.AddBool("isError", e.IsError)
	enc.AddString("attributeMap", fmt.Sprintf("%v", e.AttributeMap))
	return nil
}

// This has been copied from the exporter. The exporter needs to read from here.
type ErrorEvent struct {
	Event        Event  `json:"errorEvent,omitempty"`
	ErrorID      string `json:"errorID,omitempty"`
	ErrorGroupID string `json:"errorGroupID,omitempty"`
}

func NewEventsAndErrorEvents(input ptrace.SpanEventSlice, serviceName string, lowCardinalExceptionGrouping bool) ([]string, []ErrorEvent) {
	events := make([]string, 0)
	errorEvents := make([]ErrorEvent, 0)

	for i := 0; i < input.Len(); i++ {
		event := Event{
			Name:         input.At(i).Name(),
			TimeUnixNano: uint64(input.At(i).Timestamp()),
			AttributeMap: map[string]string{},
			IsError:      false,
		}

		input.At(i).Attributes().Range(func(k string, v pcommon.Value) bool {
			event.AttributeMap[k] = v.AsString()
			return true
		})
		errorEvent := ErrorEvent{}
		if event.Name == "exception" {
			event.IsError = true
			errorEvent.Event = event
			uuidWithHyphen := uuid.New()
			uuid := strings.Replace(uuidWithHyphen.String(), "-", "", -1)
			errorEvent.ErrorID = uuid
			var hash [16]byte
			if lowCardinalExceptionGrouping {
				hash = md5.Sum([]byte(serviceName + errorEvent.Event.AttributeMap["exception.type"]))
			} else {
				hash = md5.Sum([]byte(serviceName + errorEvent.Event.AttributeMap["exception.type"] + errorEvent.Event.AttributeMap["exception.message"]))
			}
			errorEvent.ErrorGroupID = fmt.Sprintf("%x", hash)
		}

		stringEvent, _ := json.Marshal(event)
		events = append(events, string(stringEvent))
		errorEvents = append(errorEvents, errorEvent)
	}

	return events, errorEvents
}
