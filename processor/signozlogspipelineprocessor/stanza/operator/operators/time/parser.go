// Brought in as is from opentelemetry-collector-contrib

package time

import (
	"context"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Parser is an operator that parses time from a field to an entry.
type Parser struct {
	helper.TransformerOperator
	signozstanzahelper.TimeParser
}

// Process will parse time from an entry.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.TimeParser.Parse)
}
