// Brought in as is from opentelemetry-collector-contrib
package remove

import (
	"context"
	"fmt"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Transformer is an operator that deletes a field
type Transformer struct {
	signozstanzahelper.TransformerOperator
	Field rootableField
}

// Process will process an entry with a remove transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Transform will apply the remove operation to an entry
func (t *Transformer) Transform(entry *entry.Entry) error {
	if t.Field.allAttributes {
		entry.Attributes = nil
		return nil
	}

	if t.Field.allResource {
		entry.Resource = nil
		return nil
	}

	_, exist := entry.Delete(t.Field.Field)
	if !exist {
		return fmt.Errorf("remove: field does not exist: %s", t.Field.Field.String())
	}
	return nil
}
