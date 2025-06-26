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
	jsoniter "github.com/json-iterator/go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Parser is an operator that parses JSON.
type Parser struct {
	signozstanzahelper.ParserOperator
	json               jsoniter.API
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
	switch value := value.(type) {
	case string:
		// Unquote JSON strings if possible
		err := p.json.UnmarshalFromString(utils.Unquote(value), &parsedValue)
		if err != nil {
			return nil, err
		}
	// no need to cover other map types; check comment https://github.com/SigNoz/signoz-otel-collector/pull/584#discussion_r2042020882
	case map[string]any:
		parsedValue = value
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
		if !ok {
			destination[parent] = value
			return
		}
		if depth > p.maxFlatteningDepth {
			destination[parent] = mapped
			return
		}
		// Sorting keys to have a consistent behavior when paths are not enabled and keys repeat at different levels
		keys := []string{}
		for key := range maps.Keys(mapped) {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			newKey := generateKey(parent, key)
			p.flatten(newKey, mapped[key], destination, depth+1)
		}
	default:
		destination[parent] = value
	}
}
