package json

import (
	"context"
	"fmt"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/bytedance/sonic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type Processor struct {
	signozstanzahelper.TransformerOperator
}

// Process will parse an entry for JSON.
func (p *Processor) Process(ctx context.Context, entry *entry.Entry) error {
	return p.TransformerOperator.ProcessWith(ctx, entry, p.transform)
}

func (p *Processor) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// preprocess log body
func (p *Processor) transform(entry *entry.Entry) error {
	parsedValue := make(map[string]any)
	switch v := entry.Body.(type) {
	case string:
		// Unquote JSON strings if possible
		err := sonic.Unmarshal([]byte(utils.Unquote(v)), &parsedValue)
		if err != nil { // failed to marshal as JSON; return as is
			return  nil
		}
	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		parsedValue = v
	default:
		return  fmt.Errorf("type %T cannot be parsed as JSON", v)
	}

	entry.Body = parsedValue
	return nil
}
