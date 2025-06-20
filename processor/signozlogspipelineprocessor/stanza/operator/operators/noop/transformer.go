// Brought in as is from opentelemetry-collector-contrib

package noop

import (
	"context"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Transformer is an operator that performs no operations on an entry.
type Transformer struct {
	signozstanzahelper.TransformerOperator
}

// Process will forward the entry to the next output without any alterations.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	// TODO: handle error
	_ = t.Write(ctx, entry)
	return nil
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.WriteBatch(ctx, entries)
}
