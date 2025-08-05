// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Parser is an operator that parses JSON.
type Parser struct {
	signozstanzahelper.ParserOperator
	enableFlattening   bool
	maxFlatteningDepth int
	enablePaths        bool
	pathPrefix         string
}

// Process will parse an entry for JSON.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

func (p *Parser) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return p.ProcessBatchWith(ctx, entries, p.Process)
}

// parse will parse a value as JSON.
func (p *Parser) parse(value any) (any, error) {
	parsedValue := make(map[string]any)
	switch v := value.(type) {
	case string:
		// Unquote JSON strings if possible
		err := json.Unmarshal([]byte(utils.Unquote(v)), &parsedValue)
		if err != nil {
			return nil, err
		}
	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		parsedValue = v
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as JSON", value)
	}

	if p.enableFlattening {
		flattened := make(map[string]any)
		p.flatten(p.pathPrefix, parsedValue, flattened, 0)

		return flattened, nil
	}

	return parsedValue, nil
}

func (p *Parser) flatten(parent string, value any, destination map[string]any, depth int) {
	generateKey := func(parent, key string) string {
		if p.enablePaths && parent != "" {
			return strings.Join([]string{parent, key}, ".")
		}

		return key
	}

	t := reflect.ValueOf(value)
	switch t.Kind() {
	case reflect.Map:
		mapped, ok := value.(map[string]any)
		if !ok || depth > p.maxFlatteningDepth { // if not map of string:value OR depth higher than max attach it directly and return
			destination[parent] = value
			return
		}
		// Sorting keys to have a consistent behavior when paths are not enabled and keys repeat at different levels
		keys := slices.Collect(maps.Keys(mapped))
		slices.Sort(keys)
		for _, key := range keys {
			newKey := generateKey(parent, key)
			p.flatten(newKey, mapped[key], destination, depth+1)
		}
	default:
		destination[parent] = value
	}
}
