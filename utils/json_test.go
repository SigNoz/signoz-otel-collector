package utils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeJSONFloats(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want any
	}{
		{name: "valid float unchanged", in: 3.14, want: 3.14},
		{name: "NaN becomes nil", in: math.NaN(), want: nil},
		{name: "+Inf becomes nil", in: math.Inf(1), want: nil},
		{name: "-Inf becomes nil", in: math.Inf(-1), want: nil},
		{name: "non-float scalar untouched", in: "hello", want: "hello"},
		{
			name: "map with mixed valid/invalid floats",
			in: map[string]any{
				"good": 1.0,
				"bad":  math.NaN(),
				"str":  "x",
			},
			want: map[string]any{
				"good": 1.0,
				"bad":  nil,
				"str":  "x",
			},
		},
		{
			name: "slice with mixed valid/invalid floats",
			in:   []any{1.0, math.NaN(), "x"},
			want: []any{1.0, nil, "x"},
		},
		{
			name: "deeply nested slice-of-map-of-slice",
			in: []any{
				map[string]any{
					"scores": []any{math.Inf(-1), 2.0},
				},
			},
			want: []any{
				map[string]any{
					"scores": []any{nil, 2.0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeJSONFloats(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
