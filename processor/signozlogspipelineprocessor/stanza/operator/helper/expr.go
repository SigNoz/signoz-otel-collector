// Mostly brought in as-is from otel-collector-contrib with the following changes:
// - GetExprEnv includes severity_text and severity_number
// - signozExprPatcher rewrites like/ilike AST nodes for compile-time pattern compilation

package signozstanzahelper

import (
	"errors"
	"fmt"
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

	return program, patcher.foundBodyFieldRef, patcher.compiledPatterns, patcher.error()
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

	return program, patcher.foundBodyFieldRef, patcher.compiledPatterns, patcher.error()
}

// signozExprPatcher is an expr AST visitor that runs once during expression
// compilation (i.e. at pipeline startup, not per log entry). It performs two
// independent rewrites on the AST:
//
//  1. Body field redirection — `body.x` → `body_map.x`
//  2. like/ilike pre-compilation — `like(s, "pat")` → `__like_HASH(s)`
//
// Both rewrites mutate the AST nodes in place. The expr library calls Visit
// on every node in a bottom-up traversal, so by the time expr.Compile returns,
// the compiled *vm.Program already contains the rewritten bytecode.
type signozExprPatcher struct {
	// foundBodyFieldRef is set to true when the patcher sees a reference to a
	// field inside body (e.g. body.request.id). Callers use this flag to
	// decide whether to JSON-parse the log body into body_map at runtime.
	foundBodyFieldRef bool

	// compiledPatterns is populated by the like/ilike rewrite (see Visit).
	// Keys are env slot names (e.g. "__like_3f8a1c2d"); values are the
	// corresponding pre-compiled matcher functions. This map is returned to
	// the caller by ExprCompile/ExprCompileBool and must be injected into the
	// expr env before every vm.Run call so the rewritten bytecode can resolve
	// the slot-name identifiers.
	compiledPatterns map[string]func(s string) bool

	// errs holds compile-time errors detected during the AST walk, for
	// example a like/ilike call with the wrong number of arguments or a
	// non-string literal pattern. expr.Compile itself cannot surface these
	// because AllowUndefinedVariables suppresses unknown-function errors; the
	// patcher catches them explicitly so callers get a meaningful error at
	// pipeline startup rather than at log-entry evaluation time.
	errs []error
}

// firstErr returns the first accumulated patcher error, or nil.
func (p *signozExprPatcher) error() error {
	if len(p.errs) == 0 {
		return nil
	}
	return errors.Join(p.errs...)
}

// Visit implements ast.Visitor. It is called once per AST node during
// expr.Compile and applies the two rewrites described on signozExprPatcher.
func (p *signozExprPatcher) Visit(node *ast.Node) {
	// ── Rewrite 1: body field redirection ────────────────────────────────────
	//
	// expr represents `body.request.id` as:
	//   MemberNode { Node: IdentifierNode{"body"}, Property: ... }
	//
	// We rename the root identifier from "body" to "body_map" so the
	// expression reads from the JSON-parsed body map at runtime instead of
	// the raw body string. body_map is populated by GetExprEnv when
	// foundBodyFieldRef is true.
	if memberNode, ok := (*node).(*ast.MemberNode); ok {
		if id, ok := memberNode.Node.(*ast.IdentifierNode); ok && id.Value == "body" {
			id.Value = "body_map"
			p.foundBodyFieldRef = true
		}
	}

	// The remaining rewrites only apply to function call nodes.
	n, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	c, ok := n.Callee.(*ast.IdentifierNode)
	if !ok {
		return
	}

	// ── Rewrite 2a: env() → os_env_func() ────────────────────────────────────
	//
	// The expr built-in `env` conflicts with the pipeline keyword, so we
	// renamed it to os_env_func in the env map. Mirror that here.
	if c.Value == "env" {
		c.Value = "os_env_func"
		return
	}

	// ── Rewrite 2b: like/ilike pre-compilation ────────────────────────────────
	//
	// Pipeline expressions always use constant patterns, e.g.:
	//   like(body, "%error%")
	//
	// Compiling the LIKE pattern to a *regexp.Regexp on every log entry would
	// be wasteful. Instead, we do it once here at expression compile time:
	//
	//   Step 1 — detect:  second argument must be a StringNode (compile-time
	//             constant). If it is a runtime value we leave the call alone.
	//
	//   Step 2 — compile: turn the LIKE pattern into a *regexp.Regexp and
	//             wrap it as a func(string) bool.
	//
	//   Step 3 — store:   save the matcher under a stable slot name derived
	//             from the pattern (fnv32a hash), e.g. "__like_3f8a1c2d".
	//             The slot name is added to compiledPatterns so callers can
	//             inject it into the env before vm.Run.
	//
	//   Step 4 — rewrite: mutate the CallNode so the compiled program calls
	//             __like_3f8a1c2d(body) instead of like(body, "%error%").
	//             The pattern argument is dropped because it is now baked into
	//             the closure.
	//
	// At runtime vm.Run looks up "__like_3f8a1c2d" in the env map, finds the
	// pre-compiled func(string) bool, and calls it — one RE2 scan, no allocs.
	if c.Value == "like" || c.Value == "ilike" {
		// ── Arity check ───────────────────────────────────────────────────────
		if len(n.Arguments) != 2 {
			p.errs = append(p.errs, fmt.Errorf(
				"%s() requires exactly 2 arguments, got %d", c.Value, len(n.Arguments),
			))
			return
		}

		// ── Pattern type check ────────────────────────────────────────────────
		strNode, isStr := n.Arguments[1].(*ast.StringNode)
		if !isStr {
			p.errs = append(p.errs, fmt.Errorf(
				"%s() pattern (second argument) must be a string literal", c.Value,
			))
			return
		}

		// ── Pattern compilation + rewrite ─────────────────────────────────────
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
			p.errs = append(p.errs, fmt.Errorf(
				"%s() invalid pattern %q: %w", c.Value, pattern, compileErr,
			))
			return
		}
		p.compiledPatterns[slotName] = matcher // step 3
		c.Value = slotName                     // step 4a: rename callee
		n.Arguments = n.Arguments[:1]          // step 4b: drop pattern arg
	}
}
