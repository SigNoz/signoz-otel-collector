package utils

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSanitizeJSONFloats(t *testing.T) {
	testCases := []struct {
		name     string
		in       any
		wantJSON string
	}{
		{name: "ValidFloat_Unchanged", in: 3.14, wantJSON: `3.14`},
		{name: "NaN_Null", in: math.NaN(), wantJSON: `null`},
		{name: "PosInf_Null", in: math.Inf(1), wantJSON: `null`},
		{name: "NegInf_Null", in: math.Inf(-1), wantJSON: `null`},
		{name: "NonFloatScalar_Unchanged", in: "hello", wantJSON: `"hello"`},
		{name: "WholeFloat_Unchanged", in: 200.0, wantJSON: `200`},
		{name: "Int64Range_Unchanged", in: map[string]any{"below_max": 9.2e18, "above_min": -9.2e18}, wantJSON: `{"above_min":-9200000000000000000,"below_max":9200000000000000000}`},
		{name: "LargestBelowInt64Max_Unchanged", in: math.Nextafter(1<<63, 0), wantJSON: `9223372036854775000`},
		{name: "Int64Min_Exponent", in: float64(math.MinInt64), wantJSON: `-9.223372036854776e+18`},
		{name: "BeyondUInt64_Exponent", in: 18446744073709551615.0, wantJSON: `1.8446744073709552e+19`},
		{name: "UInt64Range_Exponent", in: 1e19, wantJSON: `1e+19`},
		{name: "BelowInt64_Exponent", in: -1e19, wantJSON: `-1e+19`},
		{name: "HugeFloat_Unchanged", in: 1.5e300, wantJSON: `1.5e+300`},
		{
			name:     "Map_MixedValues",
			in:       map[string]any{"good": 1.0, "bad": math.NaN(), "big": 1e20, "str": "x"},
			wantJSON: `{"bad":null,"big":1e+20,"good":1,"str":"x"}`,
		},
		{
			name:     "Slice_MixedValues",
			in:       []any{1.0, math.NaN(), "x", -1e19},
			wantJSON: `[1,null,"x",-1e+19]`,
		},
		{
			name:     "DeeplyNested_SliceOfMapOfSlice",
			in:       []any{map[string]any{"scores": []any{math.Inf(-1), 2.0, 1e19}}},
			wantJSON: `[{"scores":[null,2,1e+19]}]`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			b, err := json.Marshal(SanitizeJSONFloats(testCase.in))
			require.NoError(t, err)
			assert.Equal(t, testCase.wantJSON, string(b))
		})
	}
}
