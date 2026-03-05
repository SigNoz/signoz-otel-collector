package utils

import "testing"

func TestLike(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		// Exact match
		{name: "exact match", s: "hello", pattern: "hello", want: true},
		{name: "exact mismatch", s: "hello", pattern: "world", want: false},
		{name: "empty both", s: "", pattern: "", want: true},
		{name: "empty string non-empty pattern", s: "", pattern: "a", want: false},
		{name: "non-empty string empty pattern", s: "a", pattern: "", want: false},

		// % wildcard
		{name: "% matches empty suffix", s: "hello", pattern: "hello%", want: true},
		{name: "% matches non-empty suffix", s: "hello world", pattern: "hello%", want: true},
		{name: "% matches empty prefix", s: "hello", pattern: "%hello", want: true},
		{name: "% matches non-empty prefix", s: "say hello", pattern: "%hello", want: true},
		{name: "% matches middle", s: "helloworld", pattern: "hello%world", want: true},
		{name: "% alone matches anything", s: "anything", pattern: "%", want: true},
		{name: "% alone matches empty", s: "", pattern: "%", want: true},
		{name: "%% is same as %", s: "abc", pattern: "%%", want: true},
		{name: "% does not match wrong prefix", s: "world", pattern: "hello%", want: false},

		// _ wildcard
		{name: "_ matches single char", s: "a", pattern: "_", want: true},
		{name: "_ does not match empty", s: "", pattern: "_", want: false},
		{name: "_ does not match two chars", s: "ab", pattern: "_", want: false},
		{name: "_ in middle", s: "abc", pattern: "a_c", want: true},
		{name: "_ in middle mismatch length", s: "ac", pattern: "a_c", want: false},
		{name: "multiple _", s: "abc", pattern: "___", want: true},
		{name: "multiple _ mismatch", s: "ab", pattern: "___", want: false},

		// Escape sequences
		{name: `\% matches literal %`, s: "100%", pattern: `100\%`, want: true},
		{name: `\% does not match regular char`, s: "100x", pattern: `100\%`, want: false},
		{name: `\_ matches literal _`, s: "a_b", pattern: `a\_b`, want: true},
		{name: `\_ does not match other char`, s: "axb", pattern: `a\_b`, want: false},
		{name: `\\ matches literal backslash`, s: `a\b`, pattern: `a\\b`, want: true},
		{name: `\\ does not match regular char`, s: "axb", pattern: `a\\b`, want: false},
		{name: `\x matches literal x`, s: "axb", pattern: `a\xb`, want: true},
		{name: `\x does not match other char`, s: "ayb", pattern: `a\xb`, want: false},

		// Combined
		{name: "% and _ combined", s: "foobar", pattern: "f%b_r", want: true},
		{name: "% at both ends", s: "needle", pattern: "%needle%", want: true},
		{name: "% at both ends with extra chars", s: "find needle here", pattern: "%needle%", want: true},
		{name: "no match with % at both ends", s: "no match here", pattern: "%needle%", want: false},

		// Case sensitivity (Like is case-sensitive)
		{name: "case sensitive mismatch", s: "Hello", pattern: "hello", want: false},
		{name: "case sensitive match", s: "hello", pattern: "hello", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Like(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("Like(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestILike(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		// Case-insensitive basics
		{name: "same case", s: "hello", pattern: "hello", want: true},
		{name: "upper string lower pattern", s: "HELLO", pattern: "hello", want: true},
		{name: "lower string upper pattern", s: "hello", pattern: "HELLO", want: true},
		{name: "mixed case", s: "HeLLo", pattern: "hElLO", want: true},

		// Wildcards still work
		{name: "% case insensitive", s: "Hello World", pattern: "hello%", want: true},
		{name: "_ case insensitive", s: "Hello", pattern: "H_llo", want: true},
		{name: "_ case insensitive upper", s: "HELLO", pattern: "h_llo", want: true},

		// Escape sequences still work
		{name: `\% literal case insensitive`, s: "50%OFF", pattern: `50\%off`, want: true},
		{name: `\_ literal case insensitive`, s: "A_B", pattern: `a\_b`, want: true},

		// Mismatch
		{name: "mismatch different word", s: "hello", pattern: "WORLD", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ILike(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("ILike(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}
