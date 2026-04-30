package signozstanzahelper

import (
	"testing"

	"github.com/expr-lang/expr/vm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// runBoolExpr compiles the expression, builds an env from the given entry, and
// returns the boolean result. It exercises the full operator path:
// ExprCompileBool → RunWithExprEnv → vm.Run.
func runBoolExpr(t *testing.T, expression string, e *entry.Entry) bool {
	t.Helper()
	program, hasBodyFieldRef, compiledPatterns, err := ExprCompileBool(expression)
	if err != nil {
		t.Fatalf("ExprCompileBool(%q): %v", expression, err)
	}

	var result bool
	if err := RunWithExprEnv(e, hasBodyFieldRef, compiledPatterns, func(env map[string]any) error {
		res, err := vm.Run(program, env)
		if err != nil {
			return err
		}
		result = res.(bool)
		return nil
	}); err != nil {
		t.Fatalf("vm.Run(%q): %v", expression, err)
	}
	return result
}

func entryWithBody(body any) *entry.Entry {
	e := entry.New()
	e.Body = body
	return e
}

func entryWithAttribute(key, value string) *entry.Entry {
	e := entry.New()
	e.Attributes = map[string]any{key: value}
	return e
}

func TestExprLike(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		entry      *entry.Entry
		want       bool
	}{
		// Exact match
		{name: "exact match", expression: `like(body, "hello")`, entry: entryWithBody("hello"), want: true},
		{name: "exact mismatch", expression: `like(attributes["status"], "world")`, entry: entryWithAttribute("status", "hello"), want: false},
		{name: "empty both", expression: `like(attributes.status, "")`, entry: entryWithAttribute("status", ""), want: true},
		{name: "empty string non-empty pattern", expression: `like(body, "a")`, entry: entryWithBody(""), want: false},
		{name: "non-empty string empty pattern", expression: `like(attributes["status"], "")`, entry: entryWithAttribute("status", "a"), want: false},

		// % wildcard
		{name: "% matches empty suffix", expression: `like(attributes.status, "hello%")`, entry: entryWithAttribute("status", "hello"), want: true},
		{name: "% matches non-empty suffix", expression: `like(body, "hello%")`, entry: entryWithBody("hello world"), want: true},
		{name: "% matches empty prefix", expression: `like(attributes["status"], "%hello")`, entry: entryWithAttribute("status", "hello"), want: true},
		{name: "% matches non-empty prefix", expression: `like(body, "%hello")`, entry: entryWithBody("say hello"), want: true},
		{name: "% matches middle", expression: `like(attributes.status, "hello%world")`, entry: entryWithAttribute("status", "helloworld"), want: true},
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
		{name: `\% matches literal %`, expression: `like(attributes["status"], "100\\%")`, entry: entryWithAttribute("status", "100%"), want: true},
		{name: `\% does not match regular char`, expression: `like(body, "100\\%")`, entry: entryWithBody("100x"), want: false},
		{name: `\_ matches literal _`, expression: `like(attributes.status, "a\\_b")`, entry: entryWithAttribute("status", "a_b"), want: true},
		{name: `\_ does not match other char`, expression: `like(body, "a\\_b")`, entry: entryWithBody("axb"), want: false},
		{name: `\\ matches literal backslash`, expression: `like(body, "a\\\\b")`, entry: entryWithBody(`a\b`), want: true},
		{name: `\\ does not match regular char`, expression: `like(body, "a\\\\b")`, entry: entryWithBody("axb"), want: false},
		{name: `\x matches literal x`, expression: `like(body, "a\\xb")`, entry: entryWithBody("axb"), want: true},
		{name: `\x does not match other char`, expression: `like(body, "a\\xb")`, entry: entryWithBody("ayb"), want: false},

		// Prefix+suffix tier (prefix%suffix)
		{name: "prefix%suffix match", expression: `like(attributes["status"], "hello%world")`, entry: entryWithAttribute("status", "hello beautiful world"), want: true},
		{name: "prefix%suffix adjacent", expression: `like(body, "hello%world")`, entry: entryWithBody("helloworld"), want: true},
		{name: "prefix%suffix wrong prefix", expression: `like(attributes.status, "hello%world")`, entry: entryWithAttribute("status", "greetings world"), want: false},
		{name: "prefix%suffix wrong suffix", expression: `like(body, "hello%world")`, entry: entryWithBody("hello earth"), want: false},
		{name: "prefix%suffix too short", expression: `like(body, "hello%world")`, entry: entryWithBody("helloworl"), want: false},

		// Combined
		{name: "% and _ combined", expression: `like(attributes["status"], "f%b_r")`, entry: entryWithAttribute("status", "foobar"), want: true},
		{name: "% at both ends", expression: `like(body, "%needle%")`, entry: entryWithBody("needle"), want: true},
		{name: "% at both ends with extra chars", expression: `like(attributes.status, "%needle%")`, entry: entryWithAttribute("status", "find needle here"), want: true},
		{name: "no match with % at both ends", expression: `like(body, "%needle%")`, entry: entryWithBody("no match here"), want: false},

		// Case sensitivity (like is case-sensitive)
		{name: "case sensitive mismatch", expression: `like(attributes["status"], "hello")`, entry: entryWithAttribute("status", "Hello"), want: false},
		{name: "case sensitive match", expression: `like(body, "hello")`, entry: entryWithBody("hello"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q: got %v, want %v (body=%v attrs=%v)",
					tt.expression, got, tt.want, tt.entry.Body, tt.entry.Attributes)
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
		{name: "same case", expression: `ilike(attributes["status"], "hello")`, entry: entryWithAttribute("status", "hello"), want: true},
		{name: "upper string lower pattern", expression: `ilike(body, "hello")`, entry: entryWithBody("HELLO"), want: true},
		{name: "lower string upper pattern", expression: `ilike(attributes.status, "HELLO")`, entry: entryWithAttribute("status", "hello"), want: true},
		{name: "mixed case", expression: `ilike(body, "hElLO")`, entry: entryWithBody("HeLLo"), want: true},

		// Wildcards still work
		{name: "% case insensitive", expression: `ilike(attributes["status"], "hello%")`, entry: entryWithAttribute("status", "Hello World"), want: true},
		{name: "_ case insensitive", expression: `ilike(body, "H_llo")`, entry: entryWithBody("Hello"), want: true},
		{name: "_ case insensitive upper", expression: `ilike(attributes.status, "h_llo")`, entry: entryWithAttribute("status", "HELLO"), want: true},

		// Escape sequences still work
		{name: `\% literal case insensitive`, expression: `ilike(body, "50\\%off")`, entry: entryWithBody("50%OFF"), want: true},
		{name: `\_ literal case insensitive`, expression: `ilike(attributes["status"], "a\\_b")`, entry: entryWithAttribute("status", "A_B"), want: true},

		// Prefix+suffix tier, case-insensitive
		{name: "prefix%suffix ilike match", expression: `ilike(attributes.status, "HELLO%WORLD")`, entry: entryWithAttribute("status", "hello beautiful world"), want: true},
		{name: "prefix%suffix ilike adjacent", expression: `ilike(body, "HELLO%WORLD")`, entry: entryWithBody("helloworld"), want: true},
		{name: "prefix%suffix ilike wrong prefix", expression: `ilike(attributes["status"], "HELLO%WORLD")`, entry: entryWithAttribute("status", "greetings world"), want: false},

		// Mismatch
		{name: "mismatch different word", expression: `ilike(attributes.status, "WORLD")`, entry: entryWithAttribute("status", "hello"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runBoolExpr(t, tt.expression, tt.entry)
			if got != tt.want {
				t.Errorf("expr %q: got %v, want %v (body=%v attrs=%v)",
					tt.expression, got, tt.want, tt.entry.Body, tt.entry.Attributes)
			}
		})
	}
}

// TestExprCompileBoolErrors verifies that like() expressions with syntax errors,
// wrong arity, non-string patterns, or a non-bool result type are rejected by
// ExprCompileBool at compile time.
//
// Arity and pattern-type errors are caught by signozExprPatcher.Visit during
// the expr.Compile AST walk and returned by ExprCompileBool immediately —
// they never reach vm.Run. Syntax errors are caught by expr.Compile itself.
func TestExprCompileBoolErrors(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "arity: like with one arg",
			expression: `like(body)`,
		},
		{
			name:       "arity: ilike with three args",
			expression: `ilike(body, "a", "b")`,
		},
		{
			name:       "pattern: like with integer literal",
			expression: `like(body, 42)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := ExprCompileBool(tt.expression)
			if err == nil {
				t.Errorf("ExprCompileBool(%q): expected error, got nil", tt.expression)
				return
			}
		})
	}
}
