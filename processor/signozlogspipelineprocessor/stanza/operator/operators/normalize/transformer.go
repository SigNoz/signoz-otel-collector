package json

import (
	"context"
	"fmt"
	"reflect"
	"sort"
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

// Step 1: if "message" missing from logs will try to extract it from msgCompatibleFields.
// msgCompatibleFields are only allocated to "message" field if they're String else skipped.
// Step 2: normalize casts "message" as String if Scalar.
// Step 3: if it's a Map, all keys will be shifted to top level and top level "message" will be removed.
func (p *Processor) normalize(entry *entry.Entry) {
	message := signozstanzaentry.NewBodyField("message")
	_, exists := entry.Get(message)
	if !exists {
		// add first found msg compatible field to body
		for _, fieldName := range msgCompatibleFields {
			field := signozstanzaentry.NewBodyField(fieldName)
			val, ok := entry.Get(field)
			if !ok {
				continue
			}
			// Only map String values to "message" field
			strValue, ok := val.(string)
			if !ok {
				continue
			}
			err := entry.Set(message, strValue)
			if err != nil {
				p.Logger().Error("Failed to set message field", zap.Error(err))
			} else {
				entry.Delete(field)
			}
			break
		}
	}

	val, exists := entry.Get(message)
	if exists {
		switch reflect.TypeOf(val).Kind() {
		case reflect.Map:
			mapValue, ok := val.(map[string]any)
			if !ok {
				p.Logger().Error("Failed to cast message field to map", zap.Any("type", fmt.Sprintf("%T", val)))
				break
			}
			// shift all keys to top level and remove "message" field
			for key, value := range mapValue {
				entry.Set(signozstanzaentry.NewBodyField(key), value)
			}
			// refetch the message field
			refetchedVal, exists := entry.Get(message)
			if exists {
				switch value := refetchedVal.(type) {
				case map[string]any:
					if len(value) == 0 {
						entry.Delete(message)
					}
				}
			}
		}
	}

	val, exists = entry.Get(message)
	if exists {
		switch reflect.TypeOf(val).Kind() {
		case reflect.Slice:
			marshaled, err := sonic.Marshal(val)
			if err != nil {
				p.Logger().Error("Failed to marshal message field", zap.Error(err))
				break
			}
			entry.Set(message, string(marshaled))
		default:
			entry.Set(message, fmt.Sprintf("%v", val))
		}
	}
}

func init() {
	sort.Strings(msgCompatibleFields)
}
