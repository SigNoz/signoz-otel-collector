package json

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/bytedance/sonic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"go.uber.org/zap"
)

const (
	MessageField = "message"
)

var msgCompatibleFields = []string{"log", "msg"}

type Processor struct {
	signozstanzahelper.TransformerOperator
	sonic.Config
}

// Process will parse an entry for JSON.
func (p *Processor) Process(ctx context.Context, entry *entry.Entry) error {
	return p.TransformerOperator.ProcessWith(ctx, entry, p.transform)
}

func (p *Processor) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// normalize log body
func (p *Processor) transform(entry *entry.Entry) error {
	parsedValue := make(map[string]any)
	switch v := entry.Body.(type) {
	case string:
		parsedValue = p.processTextLogs(v)
	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		parsedValue = v
	default:
		parsedValue = map[string]any{MessageField: v} // set to message field regardless of type
	}

	// set parsed value to body
	entry.Body = parsedValue

	p.normalize(entry)
	return nil
}

func (p *Processor) processTextLogs(str string) map[string]any {
	output := make(map[string]any)
	// Unquote JSON strings if possible
	unquoted := utils.Unquote(str)
	// if string can be a valid JSON
	if strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}") {
		dec := p.Config.Froze().NewDecoder(strings.NewReader(unquoted))
		err := dec.Decode(&output)
		if err == nil { // successfully decoded as JSON; return as is
			return output
		}
	}
	output[MessageField] = str
	return output
}

// getMessage returns the "message" field value, treating nil as missing and
// removing the nil entry so downstream steps see a clean state.
func getMessage(e *entry.Entry, field signozstanzaentry.Field) (any, bool) {
	val, exists := e.Get(field)
	if !exists {
		return nil, false
	}
	if val == nil {
		e.Delete(field)
		return nil, false
	}
	return val, true
}

// Step 1: if "message" is missing, promote the first present field from msgCompatibleFields
//
//	(any value type) into "message". The promoted value then goes through the same
//	normalization as a natively present "message" would.
//
// Step 2: if "message" is a Map, all keys are shifted to the top level and the top-level
//
//	"message" entry is removed.
//
// Note: Stringification `message` is delegated to ClickHouse to do natively with Type Hinting and
// MetadataExporter skips diving into message Slices, Maps and records strictly as String always
func (p *Processor) normalize(entry *entry.Entry) {
	message := signozstanzaentry.NewBodyField("message")

	if _, exists := getMessage(entry, message); !exists {
		// add first found msg compatible field to body
		for _, fieldName := range msgCompatibleFields {
			field := signozstanzaentry.NewBodyField(fieldName)
			val, ok := entry.Get(field)
			if !ok {
				continue
			}
			err := entry.Set(message, val)
			if err != nil {
				p.Logger().Error("Failed to set message field", zap.Error(err))
			} else {
				entry.Delete(field)
			}
			break
		}
	}

	if val, exists := getMessage(entry, message); exists {
		if reflect.TypeOf(val).Kind() == reflect.Map {
			mapValue, ok := val.(map[string]any)
			if !ok {
				p.Logger().Error("Failed to cast message field to map", zap.Any("type", fmt.Sprintf("%T", val)))
			} else {
				// delete "message" first, then shift all inner keys to top level
				entry.Delete(message)
				for key, value := range mapValue {
					entry.Set(signozstanzaentry.NewBodyField(key), value)
				}
			}
		}
	}
}
