package signozstanzahelper

import (
	"testing"
)

func TestLikeSlotName(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{
			pattern:  "foo%",
			expected: "__like_fd7dc23417835d6b",
		},
		{
			pattern:  "%bar%",
			expected: "__like_9be9e494405a145b",
		},
		{
			pattern:  "exact",
			expected: "__like_89457d67282c907",
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := likeSlotName(tt.pattern)
			if got != tt.expected {
				t.Errorf("likeSlotName(%q) = %q, want %q", tt.pattern, got, tt.expected)
			}
		})
	}
}
