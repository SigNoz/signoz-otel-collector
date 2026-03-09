package signozstanzahelper

import (
	"testing"

	"github.com/expr-lang/expr/vm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// runBoolExpr compiles the expression, builds an env from the given entry, and
// returns the boolean result. It is the same path that operators take at
// runtime, so it exercises ExprCompileBool → GetExprEnv → vm.Run end-to-end.
func runBoolExpr(t *testing.T, expression string, e *entry.Entry) bool {
	t.Helper()
	program, hasBodyFieldRef, compiledPatterns, err := ExprCompileBool(expression)
	if err != nil {
		t.Fatalf("ExprCompileBool(%q): %v", expression, err)
	}
	env := GetExprEnv(e, hasBodyFieldRef, compiledPatterns)
	defer PutExprEnv(env, compiledPatterns)

	result, err := vm.Run(program, env)
	if err != nil {
		t.Fatalf("vm.Run(%q): %v", expression, err)
	}
	return result.(bool)
}

func entryWithBody(body any) *entry.Entry {
	e := entry.New()
	e.Body = body
	return e
}

func entryWithAttrs(attrs map[string]any) *entry.Entry {
	e := entry.New()
	e.Attributes = attrs
	return e
}

// TestExprLike verifies that like() inside an expr expression produces the
// expected boolean result through the full compile → run pipeline.
func TestExprLike(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *entry.Entry
		want       bool
	}{
		// --- prefix / suffix / substring ---
		{
			name:       "prefix match",
			expression: `like(body, "hello%")`,
			entry:      entryWithBody("hello world"),
			want:       true,
		},
		{
			name:       "prefix no match",
			expression: `like(body, "hello%")`,
			entry:      entryWithBody("world hello"),
			want:       false,
		},
		{
			name:       "suffix match",
			expression: `like(body, "%world")`,
			entry:      entryWithBody("hello world"),
			want:       true,
		},
		{
			name:       "substring match",
			expression: `like(body, "%error%")`,
			entry:      entryWithBody("an error occurred"),
			want:       true,
		},
		{
			name:       "substring no match",
			expression: `like(body, "%error%")`,
			entry:      entryWithBody("everything is fine"),
			want:       false,
		},
		// --- exact match ---
		{
			name:       "exact match",
			expression: `like(body, "exact")`,
			entry:      entryWithBody("exact"),
			want:       true,
		},
		{
			name:       "exact no match",
			expression: `like(body, "exact")`,
			entry:      entryWithBody("not exact"),
			want:       false,
		},
		// --- single-char wildcard ---
		{
			name:       "underscore wildcard",
			expression: `like(body, "h_llo")`,
			entry:      entryWithBody("hello"),
			want:       true,
		},
		{
			name:       "underscore wildcard no match",
			expression: `like(body, "h_llo")`,
			entry:      entryWithBody("hllo"),
			want:       false,
		},
		// --- escape sequences ---
		// In expr string literals backslashes must be doubled, so "100\\%"
		// is the expr string 100\% which is the LIKE pattern for literal 100%.
		{
			name:       `escaped percent literal`,
			expression: `like(body, "100\\%")`,
			entry:      entryWithBody("100%"),
			want:       true,
		},
		{
			name:       `escaped percent does not match wildcard`,
			expression: `like(body, "100\\%")`,
			entry:      entryWithBody("100x"),
			want:       false,
		},
		// --- case sensitivity ---
		{
			name:       "like is case sensitive – mismatch",
			expression: `like(body, "HELLO%")`,
			entry:      entryWithBody("hello world"),
			want:       false,
		},
		// --- attribute field ---
		{
			name:       "match on attribute",
			expression: `like(attributes["service.name"], "frontend%")`,
			entry:      entryWithAttrs(map[string]any{"service.name": "frontend-api"}),
			want:       true,
		},
		// --- boolean composition ---
		{
			name:       "AND two like patterns",
			expression: `like(body, "hello%") && like(body, "%world")`,
			entry:      entryWithBody("hello world"),
			want:       true,
		},
		{
			name:       "OR two like patterns",
			expression: `like(body, "error%") || like(body, "warn%")`,
			entry:      entryWithBody("warn: disk usage high"),
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q on entry %v: got %v, want %v",
					tt.expression, tt.entry.Body, got, tt.want)
			}
		})
	}
}

// TestExprILike mirrors TestExprLike but for the case-insensitive variant.
func TestExprILike(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *entry.Entry
		want       bool
	}{
		{
			name:       "upper string lower pattern",
			expression: `ilike(body, "hello%")`,
			entry:      entryWithBody("HELLO WORLD"),
			want:       true,
		},
		{
			name:       "lower string upper pattern",
			expression: `ilike(body, "HELLO%")`,
			entry:      entryWithBody("hello world"),
			want:       true,
		},
		{
			name:       "mixed case substring",
			expression: `ilike(body, "%ERROR%")`,
			entry:      entryWithBody("An Error Occurred"),
			want:       true,
		},
		{
			name:       "ilike no match",
			expression: `ilike(body, "%error%")`,
			entry:      entryWithBody("everything is fine"),
			want:       false,
		},
		{
			name:       "ilike underscore case-insensitive",
			expression: `ilike(body, "h_llo")`,
			entry:      entryWithBody("HELLO"),
			want:       true,
		},
		{
			name:       `ilike escaped percent`,
			expression: `ilike(body, "50\\%off")`,
			entry:      entryWithBody("50%OFF"),
			want:       true,
		},
		{
			name:       "ilike on attribute",
			expression: `ilike(attributes["env"], "%PROD%")`,
			entry:      entryWithAttrs(map[string]any{"env": "us-prod-1"}),
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q on entry %v: got %v, want %v",
					tt.expression, tt.entry.Body, got, tt.want)
			}
		})
	}
}

// TestExprLikePatternIsPrecompiled checks that the patcher injected a
// compiled pattern slot (i.e. the literal pattern was recognised at compile
// time), and that reusing the same program across multiple entries works
// correctly without recompiling the pattern each time.
func TestExprLikePatternIsPrecompiled(t *testing.T) {
	const expression = `like(body, "%error%")`

	_, _, compiledPatterns, err := ExprCompileBool(expression)
	if err != nil {
		t.Fatalf("ExprCompileBool: %v", err)
	}
	if len(compiledPatterns) == 0 {
		t.Fatal("expected at least one pre-compiled pattern, got none")
	}

	// Verify the slot key follows the expected naming convention.
	for k := range compiledPatterns {
		if len(k) < 7 || k[:7] != "__like_" {
			t.Errorf("unexpected slot name %q; want prefix __like_", k)
		}
	}
}

// TestExprLikeReuseProgram verifies that a compiled *vm.Program can be reused
// across many entries — i.e. the env is properly cleaned up between runs.
func TestExprLikeReuseProgram(t *testing.T) {
	const expression = `like(body, "%error%")`

	program, hasBodyFieldRef, compiledPatterns, err := ExprCompileBool(expression)
	if err != nil {
		t.Fatalf("ExprCompileBool: %v", err)
	}

	cases := []struct {
		body string
		want bool
	}{
		{"an error occurred", true},
		{"everything fine", false},
		{"error: timeout", true},
		{"no issues", false},
		{"critical error detected", true},
	}

	for _, c := range cases {
		e := entryWithBody(c.body)
		env := GetExprEnv(e, hasBodyFieldRef, compiledPatterns)
		result, err := vm.Run(program, env)
		PutExprEnv(env, compiledPatterns)
		if err != nil {
			t.Fatalf("vm.Run on %q: %v", c.body, err)
		}
		if result.(bool) != c.want {
			t.Errorf("body=%q: got %v, want %v", c.body, result.(bool), c.want)
		}
	}
}
