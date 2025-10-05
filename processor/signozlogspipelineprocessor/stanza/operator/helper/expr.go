// Mostly brought in as-is from otel-collector-contrib with minor changes
// For example: includes severity_text and severity_number in GetExprEnv

package signozstanzahelper

import (
	"os"
	"sync"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

var envPool = sync.Pool{
	New: func() any {
		return map[string]any{
			"os_env_func": os.Getenv,
			"isJSON": func(v any) bool {
				return utils.IsJSON(v)
			},
			"unquote": func(v any) string {
				return utils.Unquote(v.(string))
			},
		}
	},
}

// GetExprEnv returns a map of key/value pairs that can be be used to evaluate an expression
func GetExprEnv(e *entry.Entry, forExprWithBodyFieldRef bool) map[string]any {
	env := envPool.Get().(map[string]any)
	env["$"] = e.Body
	env["body"] = e.Body
	env["attributes"] = e.Attributes
	env["resource"] = e.Resource
	env["timestamp"] = e.Timestamp
	env["severity_text"] = e.SeverityText
	env["severity_number"] = int(e.Severity)
	env["trace_id"] = e.TraceID
	env["span_id"] = e.SpanID
	env["trace_flags"] = e.TraceFlags

	if forExprWithBodyFieldRef {
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
	patcher := &signozExprPatcher{}
	program, err = expr.Compile(
		input, expr.AllowUndefinedVariables(), expr.Patch(patcher),
	)
	if err != nil {
		return nil, false, err
	}

	return program, patcher.foundBodyFieldRef, err
}

func ExprCompileBool(input string) (
	program *vm.Program, hasBodyFieldRef bool, err error,
) {
	patcher := &signozExprPatcher{}
	program, err = expr.Compile(
		input, expr.AllowUndefinedVariables(), expr.Patch(patcher), expr.AsBool(),
	)
	if err != nil {
		return nil, false, err
	}

	return program, patcher.foundBodyFieldRef, err
}

type signozExprPatcher struct {
	// set to true if the patcher encounters a reference to a body field
	// (like `body.request.id`) while compiling an expression
	foundBodyFieldRef bool
}

func (p *signozExprPatcher) Visit(node *ast.Node) {
	// Change all references to fields inside body (eg: body.request.id)
	// to refer inside body_map instead (eg: body_map.request.id)
	//
	// `body_map` is supplied in expr env's by JSON parsing the log body
	// when it contains serialized JSON
	memberAccessNode, isMemberNode := (*node).(*ast.MemberNode)
	if isMemberNode {
		// MemberNode represents a member access in expr
		// It can be a field access, a method call, or an array element access.
		// Example: `foo.bar` or `foo["bar"]` or `foo.bar()` or `array[0]`
		//
		// `memberNode.Node` is the node whose property/member is being accessed.
		// Eg: the node for `body` in `body.request.id`
		//
		// `memberNode.Property` is the property being accessed on `memberNode.Node`
		// Eg: the AST for `request.id` in `body.request.id`
		//
		// Change all `MemberNode`s where the target (`memberNode.Node`)
		// is `body` to target `body_map` instead.
		identifierNode, isIdentifierNode := (memberAccessNode.Node).(*ast.IdentifierNode)
		if isIdentifierNode && identifierNode.Value == "body" {
			identifierNode.Value = "body_map"
			p.foundBodyFieldRef = true
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
