package signozstanzahelper

import (
	"testing"

	"github.com/expr-lang/expr/vm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// runBoolExpr compiles the expression, builds an env from the given entry, and
// returns the boolean result. It exercises the full operator path:
// ExprCompileBool → GetExprEnv → vm.Run → PutExprEnv.
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

// Note on escape sequences in expr string literals:
// expr parses string literals with the same escape rules as Go, so a single
// backslash in the LIKE pattern must be written as \\ inside the expr string.
// Examples:
//   LIKE pattern  │  expr string literal
//   ─────────────────────────────────────
//   100\%         │  "100\\%"
//   a\_b          │  "a\\_b"
//   a\\b          │  "a\\\\b"
//   a\xb          │  "a\\xb"

func TestExprLike(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *entry.Entry
		want       bool
	}{
		// Exact match
		{name: "exact match", expression: `like(body, "hello")`, entry: entryWithBody("hello"), want: true},
		{name: "exact mismatch", expression: `like(body, "world")`, entry: entryWithBody("hello"), want: false},
		{name: "empty both", expression: `like(body, "")`, entry: entryWithBody(""), want: true},
		{name: "empty string non-empty pattern", expression: `like(body, "a")`, entry: entryWithBody(""), want: false},
		{name: "non-empty string empty pattern", expression: `like(body, "")`, entry: entryWithBody("a"), want: false},

		// % wildcard
		{name: "% matches empty suffix", expression: `like(body, "hello%")`, entry: entryWithBody("hello"), want: true},
		{name: "% matches non-empty suffix", expression: `like(body, "hello%")`, entry: entryWithBody("hello world"), want: true},
		{name: "% matches empty prefix", expression: `like(body, "%hello")`, entry: entryWithBody("hello"), want: true},
		{name: "% matches non-empty prefix", expression: `like(body, "%hello")`, entry: entryWithBody("say hello"), want: true},
		{name: "% matches middle", expression: `like(body, "hello%world")`, entry: entryWithBody("helloworld"), want: true},
		{name: "% alone matches anything", expression: `like(body, "%")`, entry: entryWithBody("anything"), want: true},
		{name: "% alone matches empty", expression: `like(body, "%")`, entry: entryWithBody(""), want: true},
		{name: "%% is same as %", expression: `like(body, "%%")`, entry: entryWithBody("abc"), want: true},
		{name: "% does not match wrong prefix", expression: `like(body, "hello%")`, entry: entryWithBody("world"), want: false},

		// _ wildcard
		{name: "_ matches single char", expression: `like(body, "_")`, entry: entryWithBody("a"), want: true},
		{name: "_ does not match empty", expression: `like(body, "_")`, entry: entryWithBody(""), want: false},
		{name: "_ does not match two chars", expression: `like(body, "_")`, entry: entryWithBody("ab"), want: false},
		{name: "_ in middle", expression: `like(body, "a_c")`, entry: entryWithBody("abc"), want: true},
		{name: "_ in middle mismatch length", expression: `like(body, "a_c")`, entry: entryWithBody("ac"), want: false},
		{name: "multiple _", expression: `like(body, "___")`, entry: entryWithBody("abc"), want: true},
		{name: "multiple _ mismatch", expression: `like(body, "___")`, entry: entryWithBody("ab"), want: false},

		// Escape sequences
		{name: `\% matches literal %`, expression: `like(body, "100\\%")`, entry: entryWithBody("100%"), want: true},
		{name: `\% does not match regular char`, expression: `like(body, "100\\%")`, entry: entryWithBody("100x"), want: false},
		{name: `\_ matches literal _`, expression: `like(body, "a\\_b")`, entry: entryWithBody("a_b"), want: true},
		{name: `\_ does not match other char`, expression: `like(body, "a\\_b")`, entry: entryWithBody("axb"), want: false},
		{name: `\\ matches literal backslash`, expression: `like(body, "a\\\\b")`, entry: entryWithBody(`a\b`), want: true},
		{name: `\\ does not match regular char`, expression: `like(body, "a\\\\b")`, entry: entryWithBody("axb"), want: false},
		{name: `\x matches literal x`, expression: `like(body, "a\\xb")`, entry: entryWithBody("axb"), want: true},
		{name: `\x does not match other char`, expression: `like(body, "a\\xb")`, entry: entryWithBody("ayb"), want: false},

		// Combined
		{name: "% and _ combined", expression: `like(body, "f%b_r")`, entry: entryWithBody("foobar"), want: true},
		{name: "% at both ends", expression: `like(body, "%needle%")`, entry: entryWithBody("needle"), want: true},
		{name: "% at both ends with extra chars", expression: `like(body, "%needle%")`, entry: entryWithBody("find needle here"), want: true},
		{name: "no match with % at both ends", expression: `like(body, "%needle%")`, entry: entryWithBody("no match here"), want: false},

		// Case sensitivity (like is case-sensitive)
		{name: "case sensitive mismatch", expression: `like(body, "hello")`, entry: entryWithBody("Hello"), want: false},
		{name: "case sensitive match", expression: `like(body, "hello")`, entry: entryWithBody("hello"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q on body %v: got %v, want %v",
					tt.expression, tt.entry.Body, got, tt.want)
			}
		})
	}
}

func TestExprILike(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *entry.Entry
		want       bool
	}{
		// Case-insensitive basics
		{name: "same case", expression: `ilike(body, "hello")`, entry: entryWithBody("hello"), want: true},
		{name: "upper string lower pattern", expression: `ilike(body, "hello")`, entry: entryWithBody("HELLO"), want: true},
		{name: "lower string upper pattern", expression: `ilike(body, "HELLO")`, entry: entryWithBody("hello"), want: true},
		{name: "mixed case", expression: `ilike(body, "hElLO")`, entry: entryWithBody("HeLLo"), want: true},

		// Wildcards still work
		{name: "% case insensitive", expression: `ilike(body, "hello%")`, entry: entryWithBody("Hello World"), want: true},
		{name: "_ case insensitive", expression: `ilike(body, "H_llo")`, entry: entryWithBody("Hello"), want: true},
		{name: "_ case insensitive upper", expression: `ilike(body, "h_llo")`, entry: entryWithBody("HELLO"), want: true},

		// Escape sequences still work
		{name: `\% literal case insensitive`, expression: `ilike(body, "50\\%off")`, entry: entryWithBody("50%OFF"), want: true},
		{name: `\_ literal case insensitive`, expression: `ilike(body, "a\\_b")`, entry: entryWithBody("A_B"), want: true},

		// Mismatch
		{name: "mismatch different word", expression: `ilike(body, "WORLD")`, entry: entryWithBody("hello"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q on body %v: got %v, want %v",
					tt.expression, tt.entry.Body, got, tt.want)
			}
		})
	}
}

// TestExprLikePatternIsPrecompiled asserts that the patcher detected the
// literal pattern at compile time and injected a compiled slot rather than
// leaving a runtime like() call.
func TestExprLikePatternIsPrecompiled(t *testing.T) {
	const expression = `like(body, "%error%")`

	_, _, compiledPatterns, err := ExprCompileBool(expression)
	if err != nil {
		t.Fatalf("ExprCompileBool: %v", err)
	}
	if len(compiledPatterns) == 0 {
		t.Fatal("expected at least one pre-compiled pattern, got none")
	}
	for k := range compiledPatterns {
		if len(k) < 7 || k[:7] != "__like_" {
			t.Errorf("unexpected slot name %q; want prefix __like_", k)
		}
	}
}

// TestExprLikeReuseProgram compiles once and runs the same *vm.Program over
// multiple entries, verifying that env pool cleanup between calls is correct.
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
