package llmpricing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatternMatches(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		model   string
		want    bool
	}{
		{name: "Exact", pattern: "claude-opus-5-5", model: "claude-opus-5-5", want: true},
		{name: "DatedSnapshot", pattern: "claude-opus-5-5", model: "claude-opus-5-5-20260921", want: true},
		{name: "DashedDate", pattern: "gpt-5", model: "gpt-5-2025-08-07", want: true},
		{name: "ShortDate", pattern: "gpt-4o", model: "gpt-4o-05-13", want: true},
		{name: "MonthYearDate", pattern: "command-a", model: "command-a-03-2025", want: true},
		{name: "StableVersion", pattern: "gemini-2.0-flash", model: "gemini-2.0-flash-001", want: true},
		{name: "LatestAlias", pattern: "claude-opus-5-5", model: "claude-opus-5-5-latest", want: true},
		{name: "BedrockVersion", pattern: "claude-opus-5-5", model: "claude-opus-5-5-20260921-v1:0", want: true},
		{name: "VertexVersion", pattern: "claude-opus-5-5", model: "claude-opus-5-5@20260921", want: true},
		{name: "VertexDefault", pattern: "gemini-2.5-pro", model: "gemini-2.5-pro@default", want: true},
		{name: "MinorVersionBump_NoMatch", pattern: "claude-opus-5", model: "claude-opus-5-5", want: false},
		{name: "SiblingModel_NoMatch", pattern: "gpt-5", model: "gpt-5-mini-2025-08-07", want: false},
		{name: "DottedVersionBump_NoMatch", pattern: "gpt-5", model: "gpt-5.1-2025-11-13", want: false},
		{name: "ExtraDigitVersion_NoMatch", pattern: "grok-build-0.1", model: "grok-build-0.10", want: false},
		{name: "ShorterThanPrefix_NoMatch", pattern: "grok-build-0.1", model: "grok-build-0", want: false},
		{name: "DifferentCase_NoMatch", pattern: "gpt-5", model: "GPT-5", want: false},
		{name: "TrailingStar_AnySuffix", pattern: "gpt-4o*", model: "gpt-4o-mini", want: true},
		{name: "StarOnly_MatchesEverything", pattern: "*", model: "anything", want: true},
		{name: "BracketLiteral", pattern: "claude-sonnet-4-5[1m]", model: "claude-sonnet-4-5[1m]", want: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, ParsePattern(testCase.pattern).Matches(testCase.model))
		})
	}
}

func TestValidatePattern(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{name: "Plain", pattern: "gpt-4o"},
		{name: "TrailingStar", pattern: "gpt-4o*"},
		{name: "StarOnly", pattern: "*"},
		{name: "BracketLiteral", pattern: "claude-sonnet-4-5[1m]"},
		{name: "Empty", pattern: "", wantErr: true},
		{name: "MidStar", pattern: "gpt-*-turbo", wantErr: true},
		{name: "DoubleStar", pattern: "gpt-4o**", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidatePattern(testCase.pattern)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
