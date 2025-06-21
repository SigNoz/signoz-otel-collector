// Brought in as is from opentelemetry-collector-contrib

package add

import (
	"context"
	"fmt"
	"strings"

	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/expr-lang/expr/vm"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Transformer is an operator that adds a string value or an expression value
type Transformer struct {
	signozstanzahelper.TransformerOperator

	Field                    entry.Field
	Value                    any
	program                  *vm.Program
	valueExprHasBodyFieldRef bool
}

// Process will process an entry with a add transformation.
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.Transform)
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWith(ctx, entries, t.Process)
}

// Transform will apply the add operations to an entry
func (t *Transformer) Transform(e *entry.Entry) error {
	if t.Value != nil {
		return e.Set(t.Field, t.Value)
	}
	if t.program != nil {
		env := signozstanzahelper.GetExprEnv(e, t.valueExprHasBodyFieldRef)
		defer signozstanzahelper.PutExprEnv(env)

		result, err := vm.Run(t.program, env)
		if err != nil {
			return fmt.Errorf("evaluate value_expr: %w", err)
		}
		return e.Set(t.Field, result)
	}
	return fmt.Errorf("add: missing required field 'value'")
}

func isExpr(str string) bool {
	return strings.HasPrefix(str, "EXPR(") && strings.HasSuffix(str, ")")
}
