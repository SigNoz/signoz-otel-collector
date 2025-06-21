// Brought in as is from opentelemetry-collector-contrib

package move

import (
	"context"
	"fmt"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Transformer is an operator that moves a field's value to a new field
type Transformer struct {
	signozstanzahelper.TransformerOperator
	From entry.Field
	To   entry.Field
}

// Process will process an entry with a move transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Transform will apply the move operation to an entry
func (t *Transformer) Transform(e *entry.Entry) error {
	val, exist := t.From.Delete(e)
	if !exist {
		return fmt.Errorf("move: field does not exist: %s", t.From.String())
	}
	return t.To.Set(e, val)
}
