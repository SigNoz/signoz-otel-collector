package utils

import (
	"testing"
)

func TestToLookUpMap_String(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantKeys []string
	}{
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "c", "a"},
			wantKeys: []string{"a", "b", "c"},
		},
		{
			name:     "single element",
			input:    []string{"x"},
			wantKeys: []string{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLookUpMap(tt.input)
			if len(result) != len(tt.wantKeys) {
				t.Fatalf("expected %d entries, got %d", len(tt.wantKeys), len(result))
			}
			for _, k := range tt.wantKeys {
				if _, ok := result[k]; !ok {
					t.Errorf("expected key %q in map", k)
				}
			}
		})
	}
}

func TestToLookUpMap_Int(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		wantKeys []int
	}{
		{
			name:     "with duplicates",
			input:    []int{1, 2, 3, 1},
			wantKeys: []int{1, 2, 3},
		},
		{
			name:     "single element",
			input:    []int{42},
			wantKeys: []int{42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLookUpMap(tt.input)
			if len(result) != len(tt.wantKeys) {
				t.Fatalf("expected %d entries, got %d", len(tt.wantKeys), len(result))
			}
			for _, k := range tt.wantKeys {
				if _, ok := result[k]; !ok {
					t.Errorf("expected key %d in map", k)
				}
			}
		})
	}
}
