// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package keyvalue // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/keyvalue"

import (
	"context"
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/otel-collector-contrib-internal/parseutils"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

// Parser is an operator that parses key value pairs.
type Parser struct {
	helper.ParserOperator
	delimiter     string
	pairDelimiter string
}

// Process will parse an entry for key value pairs.
func (p *Parser) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ParserOperator.ProcessWith(ctx, entry, p.parse)
}

// parse will parse a value as key values.
func (p *Parser) parse(value any) (any, error) {
	switch m := value.(type) {
	case string:
		return p.parser(m, p.delimiter, p.pairDelimiter)
	default:
		return nil, fmt.Errorf("type %T cannot be parsed as key value pairs", value)
	}
}

func (p *Parser) parser(input string, delimiter string, pairDelimiter string) (map[string]any, error) {
	if input == "" {
		return nil, fmt.Errorf("parse from field %s is empty", p.ParseFrom.String())
	}

	pairs, err := parseutils.SplitString(input, pairDelimiter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pairs from input: %w", err)
	}

	return parseutils.ParseKeyValuePairs(pairs, delimiter)
}
