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

// GetExprEnv returns a map of key/value pairs that can be used to evaluate an
// expression. compiledPatterns contains pre-compiled like/ilike matchers
// produced by ExprCompile; pass nil if the expression has none.
func GetExprEnv(e *entry.Entry, forExprWithBodyFieldRef bool, compiledPatterns map[string]func(s string) bool) map[string]any {
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

	for k, v := range compiledPatterns {
		env[k] = v
	}

	return env
}

// PutExprEnv returns an env map to the pool. compiledPatterns must be the same
// map that was passed to GetExprEnv so that the per-expression slots are
// removed before the map is reused.
func PutExprEnv(e map[string]any, compiledPatterns map[string]func(s string) bool) {
	for k := range compiledPatterns {
		delete(e, k)
	}
	envPool.Put(e)
}

// ExprCompile compiles an expression and returns the program, whether it
// references body fields, a map of pre-compiled like/ilike pattern matchers
// keyed by their env slot names, and any compilation error.
func ExprCompile(input string) (
	program *vm.Program, hasBodyFieldRef bool, compiledPatterns map[string]func(s string) bool, err error,
) {
	patcher := &signozExprPatcher{compiledPatterns: make(map[string]func(s string) bool)}
	program, err = expr.Compile(
		input, expr.AllowUndefinedVariables(), expr.Patch(patcher),
	)
	if err != nil {
		return nil, false, nil, err
	}

	return program, patcher.foundBodyFieldRef, patcher.compiledPatterns, nil
}

// ExprCompileBool is like ExprCompile but additionally requires the expression
// to evaluate to a boolean.
func ExprCompileBool(input string) (
	program *vm.Program, hasBodyFieldRef bool, compiledPatterns map[string]func(s string) bool, err error,
) {
	patcher := &signozExprPatcher{compiledPatterns: make(map[string]func(s string) bool)}
	program, err = expr.Compile(
		input, expr.AllowUndefinedVariables(), expr.Patch(patcher), expr.AsBool(),
	)
	if err != nil {
		return nil, false, nil, err
	}

	return program, patcher.foundBodyFieldRef, patcher.compiledPatterns, nil
}

type signozExprPatcher struct {
	// set to true if the patcher encounters a reference to a body field
	// (like `body.request.id`) while compiling an expression
	foundBodyFieldRef bool
	// compiledPatterns holds pre-compiled like/ilike matchers keyed by their
	// env slot name (e.g. "__like_a1b2c3d4"). The patcher populates this map
	// when it finds a like/ilike call whose pattern argument is a string
	// literal; the call is then rewritten to use the slot-name function so
	// that no pattern compilation happens at log-processing time.
	compiledPatterns map[string]func(s string) bool
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
		return
	}

	// Pre-compile like/ilike calls whose pattern argument is a string literal.
	// like(s, "pattern")  → __like_HASH(s)
	// ilike(s, "pattern") → __ilike_HASH(s)
	// The generic like/ilike functions remain in the env as a fallback for
	// the rare case where the pattern is a runtime (non-literal) value.
	if (c.Value == "like" || c.Value == "ilike") && len(n.Arguments) == 2 {
		strNode, isStr := n.Arguments[1].(*ast.StringNode)
		if !isStr {
			return
		}
		pattern := strNode.Value
		var matcher func(string) bool
		var slotName string
		var compileErr error
		if c.Value == "like" {
			matcher, compileErr = compileLike(pattern)
			slotName = likeSlotName(pattern)
		} else {
			matcher, compileErr = compileILike(pattern)
			slotName = iLikeSlotName(pattern)
		}
		if compileErr != nil {
			// Leave the call as-is; the generic function will handle it.
			return
		}
		p.compiledPatterns[slotName] = matcher
		c.Value = slotName
		n.Arguments = n.Arguments[:1] // drop the now-baked-in pattern arg
	}
}
