package utils

import (
	"math"
	"testing"

	"github.com/goccy/go-json"
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

func TestSanitizeJSON(t *testing.T) {
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
			b, err := json.Marshal(SanitizeJSON(testCase.in))
			require.NoError(t, err)
			assert.Equal(t, testCase.wantJSON, string(b))
		})
	}
}

func nestMaps(levels int, leaf map[string]any) map[string]any {
	m := leaf
	for i := 1; i < levels; i++ {
		m = map[string]any{"a": m}
	}
	return m
}

func nestSlices(levels int, leaf any) any {
	v := leaf
	for i := 0; i < levels; i++ {
		v = []any{v}
	}
	return v
}

// jsonShape returns the container depth, the longest run of directly nested slices, and the deepest scalar.
func jsonShape(v any, arrayRun int) (int, int, any) {
	switch val := v.(type) {
	case map[string]any:
		depth, maxRun, leaf := 1, 0, any(nil)
		for _, child := range val {
			d, r, l := jsonShape(child, 0)
			if d+1 >= depth {
				depth, leaf = d+1, l
			}
			maxRun = max(maxRun, r)
		}
		return depth, maxRun, leaf
	case []any:
		depth, maxRun, leaf := 1, arrayRun+1, any(nil)
		for _, child := range val {
			d, r, l := jsonShape(child, arrayRun+1)
			if d+1 >= depth {
				depth, leaf = d+1, l
			}
			maxRun = max(maxRun, r)
		}
		return depth, maxRun, leaf
	default:
		return 0, 0, v
	}
}

func TestSanitizeJSONDepthLimits(t *testing.T) {
	testCases := []struct {
		name         string
		in           func() any
		wantDepth    int
		wantArrayRun int
		wantDeepest  any
	}{
		{
			name:        "Objects_AtDepthLimit_Unchanged",
			in:          func() any { return nestMaps(1000, map[string]any{"leaf": int64(1)}) },
			wantDepth:   1000,
			wantDeepest: int64(1),
		},
		{
			name:        "Objects_PastDepthLimit_DeepestObjectStringified",
			in:          func() any { return nestMaps(1001, map[string]any{"leaf": int64(1)}) },
			wantDepth:   1000,
			wantDeepest: `{"leaf":1}`,
		},
		{
			name:        "Objects_FarPastDepthLimit_RemainderStringifiedOnce",
			in:          func() any { return nestMaps(1003, map[string]any{"leaf": int64(1)}) },
			wantDepth:   1000,
			wantDeepest: `{"a":{"a":{"leaf":1}}}`,
		},
		{
			name:         "Arrays_AtArrayLimit_Unchanged",
			in:           func() any { return map[string]any{"a": nestSlices(128, int64(1))} },
			wantDepth:    129,
			wantArrayRun: 128,
			wantDeepest:  int64(1),
		},
		{
			name:         "Arrays_PastArrayLimit_InnermostStringified",
			in:           func() any { return map[string]any{"a": nestSlices(130, int64(1))} },
			wantDepth:    129,
			wantArrayRun: 128,
			wantDeepest:  `[[1]]`,
		},
		{
			name: "Arrays_RunResetsInsideObject_Unchanged",
			in: func() any {
				return map[string]any{"a": nestSlices(100, map[string]any{"b": nestSlices(100, int64(1))})}
			},
			wantDepth:    202,
			wantArrayRun: 100,
			wantDeepest:  int64(1),
		},
		{
			name: "Stringified_SubtreeFloatsSanitized",
			in: func() any {
				return map[string]any{"a": nestSlices(129, map[string]any{"nan": math.NaN(), "big": 1e20})}
			},
			wantDepth:    129,
			wantArrayRun: 128,
			wantDeepest:  `[{"big":1e+20,"nan":null}]`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := SanitizeJSON(testCase.in())
			depth, arrayRun, deepest := jsonShape(got, 0)
			assert.Equal(t, testCase.wantDepth, depth, "depth")
			assert.Equal(t, testCase.wantArrayRun, arrayRun, "array run")
			assert.Equal(t, testCase.wantDeepest, deepest, "deepest value")
			_, err := json.Marshal(got)
			assert.NoError(t, err)
		})
	}
}
