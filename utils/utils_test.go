package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func IsJSONB(str string) bool {
	return json.Valid([]byte(str))
}

// Generates a JSON string of the form: {"key": "value", "key1": "value1", ...}
// up to the specified size in bytes (approximate, not exact)
func generateJSON(size int) string {
	builder := strings.Builder{}
	builder.WriteString("{")
	keyCount := 0
	for builder.Len() < size-20 { // -20 to account for braces and trailing comma
		builder.WriteString(`"key`)
		builder.WriteString(string('A' + rune(keyCount%26)))
		builder.WriteString(`":"value`)
		builder.WriteString(string('A' + rune(keyCount%26)))
		builder.WriteString(`",`)
		keyCount++
	}
	s := builder.String()
	s = strings.TrimRight(s, ",") + "}"
	return s
}

var sizes = []int{
	10,      // very small
	100,     // small
	1_000,   // medium
	10_000,  // large
	100_000, // very large
}

func BenchmarkIsJSON(b *testing.B) {
	for _, size := range sizes {
		b.Run("Size_"+itoa(size), func(b *testing.B) {
			jsonStr := generateJSON(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = IsJSON(jsonStr)
			}
		})
	}
}

func BenchmarkIsJSONB(b *testing.B) {
	for _, size := range sizes {
		b.Run("Size_"+itoa(size), func(b *testing.B) {
			jsonStr := generateJSON(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = IsJSONB(jsonStr)
			}
		})
	}
}

// Lightweight integer to string (avoids fmt)
func itoa(n int) string {
	return strings.Trim(strings.ReplaceAll(strings.Trim(fmt.Sprintf("%d", n), " "), " ", "_"), "_")
}
