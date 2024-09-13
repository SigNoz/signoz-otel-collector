// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/transformer/filter"

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/expr-lang/expr/vm"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/helper"
)

// Transformer is an operator that filters entries based on matching expressions
type Transformer struct {
	helper.TransformerOperator
	expression *vm.Program
	dropCutoff *big.Int // [0..1000)
}

// Process will drop incoming entries that match the filter expression
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	env := helper.GetExprEnv(entry)
	defer helper.PutExprEnv(env)

	matches, err := vm.Run(t.expression, env)
	if err != nil {
		t.Logger().Error("Running expressing returned an error", zap.Error(err))
		return nil
	}

	filtered, ok := matches.(bool)
	if !ok {
		t.Logger().Error("Expression did not compile as a boolean")
		return nil
	}

	if !filtered {
		t.Write(ctx, entry)
		return nil
	}

	i, err := randInt(rand.Reader, upperBound)
	if err != nil {
		return err
	}

	if i.Cmp(t.dropCutoff) >= 0 {
		t.Write(ctx, entry)
	}

	return nil
}
