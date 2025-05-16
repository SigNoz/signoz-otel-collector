// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"context"
	"fmt"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	jsoniter "github.com/json-iterator/go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Parser is an operator that parses JSON.
type Parser struct {
	signozstanzahelper.ParserOperator
	json jsoniter.API
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// Process will parse an entry for JSON.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a value as JSON.
func (p *Parser) parse(value any) (any, error) {
	parsedValue := make(map[string]any)
	switch m := value.(type) {
	case string:
		err := p.json.UnmarshalFromString(m, &parsedValue)
		if err != nil {
			return nil, err
		}
	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		return m, nil
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}
	return parsedValue, nil
}
