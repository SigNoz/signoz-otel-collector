package json

import (
	"context"
	"strings"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/bytedance/sonic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// var msgCompatibleFields = []string{"msg"}

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
		// Unquote JSON strings if possible
		unquoted := utils.Unquote(v)
		if !(strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}")) {
			return nil
		}
		dec := p.Config.Froze().NewDecoder(strings.NewReader(unquoted))
		err := dec.Decode(&parsedValue)
		if err != nil { // failed to marshal as JSON; return as is
			return nil
		}

	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		parsedValue = v
	default:
		return nil // return nil to avoid erroring out the pipeline
	}

	// set parsed value to body
	entry.Body = parsedValue

	// TODO: re-enable in PipelinesV2
	// messageField := signozstanzaentry.NewBodyField("message")
	// // add first found msg compatible field to body
	// for _, fieldName := range msgCompatibleFields {
	// 	field := signozstanzaentry.NewBodyField(fieldName)
	// 	val, ok := entry.Get(field)
	// 	if !ok {
	// 		continue
	// 	}
	// 	strValue, ok := val.(string)
	// 	if !ok {
	// 		continue
	// 	}
	// 	err := entry.Set(messageField, strValue)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	entry.Delete(field)
	// 	break
	// }

	return nil
}

// func init() {
// 	sort.Strings(msgCompatibleFields)
// }
