package json

import (
	"context"
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

	messageField := signozstanzaentry.NewBodyField("message")
	// add first found msg compatible field to body
	for _, fieldName := range msgCompatibleFields {
		field := signozstanzaentry.NewBodyField(fieldName)
		val, ok := entry.Get(field)
		if !ok {
			continue
		}
		strValue, ok := val.(string)
		if !ok {
			continue
		}
		err := entry.Set(messageField, strValue)
		if err != nil {
			p.Logger().Error("Failed to set message field", zap.Error(err))
		} else {
			entry.Delete(field)
		}
		break
	}

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

func init() {
	sort.Strings(msgCompatibleFields)
}
