// Brought in as is from opentelemetry-collector-contrib

package copy

import (
	"context"
	"fmt"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Transformer copies a value from one field and creates a new field with that value
type Transformer struct {
	signozstanzahelper.TransformerOperator
	From signozstanzaentry.Field
	To   entry.Field
}

// Process will process an entry with a copy transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Transform will apply the copy operation to an entry
func (t *Transformer) Transform(e *entry.Entry) error {
	val, exist := t.From.Get(e)
	if !exist {
		return fmt.Errorf("copy: from field does not exist in this entry: %s", t.From.String())
	}
	return t.To.Set(e, val)
}
