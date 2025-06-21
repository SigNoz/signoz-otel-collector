// Brought in as is from opentelemetry-collector-contrib

package trace

import (
	"context"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Config is an operator that parses traces from fields to an entry.
type Parser struct {
	signozstanzahelper.TransformerOperator
	signozstanzahelper.TraceParser
}

// Process will parse traces from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}
