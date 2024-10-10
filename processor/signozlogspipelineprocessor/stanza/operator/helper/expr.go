// Mostly brought in as-is from otel-collector-contrib with minor changes
// For example: includes severity_text and severity_number in GetExprEnv

package signozstanzahelper

import (
	"os"
	"sync"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

var envPool = sync.Pool{
	New: func() any {
		return map[string]any{
			"os_env_func": os.Getenv,
		}
	},
}

// GetExprEnv returns a map of key/value pairs that can be be used to evaluate an expression
func GetExprEnv(e *entry.Entry, includeBodyMap bool) map[string]any {
	env := envPool.Get().(map[string]any)
	env["$"] = e.Body
	env["body"] = e.Body
	env["attributes"] = e.Attributes
	env["resource"] = e.Resource
	env["timestamp"] = e.Timestamp
	env["severity_text"] = e.SeverityText
	env["severity_number"] = int(e.Severity)

	if includeBodyMap {
		env["body_map"] = signozstanzaentry.ParseBodyJson(e)
	}

	return env
}

// PutExprEnv adds a key/value pair that will can be used to evaluate an expression
func PutExprEnv(e map[string]any) {
	envPool.Put(e)
}

func ExprCompile(input string) (
	program *vm.Program, hasBodyFieldRef bool, err error,
) {
	patcher := &exprPatcher{}
	program, err = expr.Compile(input, expr.AllowUndefinedVariables(), expr.Patch(patcher))
	if err != nil {
		return nil, false, err
	}
	return program, patcher.hasBodyFieldRef, err
}

func ExprCompileBool(input string) (
	program *vm.Program, hasBodyFieldRef bool, err error,
) {
	patcher := &exprPatcher{}
	program, err = expr.Compile(input, expr.AllowUndefinedVariables(), expr.Patch(patcher), expr.AsBool())
	if err != nil {
		return nil, false, err
	}
	return program, patcher.hasBodyFieldRef, err
}

type exprPatcher struct {
	hasBodyFieldRef bool
}

func (p *exprPatcher) Visit(node *ast.Node) {
	// Change all references to fields inside body
	// to refer inside bodyMap (supplied in expr env)
	memberAccess, ok := (*node).(*ast.MemberNode)
	if ok {
		identifier, ok := (memberAccess.Node).(*ast.IdentifierNode)
		if ok && identifier.Value == "body" {
			identifier.Value = "body_map"
			p.hasBodyFieldRef = true
		}
	}

	n, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	c, ok := (n.Callee).(*ast.IdentifierNode)
	if !ok {
		return
	}
	if c.Value == "env" {
		c.Value = "os_env_func"
	}
}
