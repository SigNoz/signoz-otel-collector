package signozlogsnormalizerprocessor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestStashMatchesAsString(t *testing.T) {
	testCases := []struct {
		name string
		body map[string]any
	}{
		{name: "EmptyMap", body: map[string]any{}},
		{name: "KeysSorted", body: map[string]any{"z": "1", "a": "2", "Z": "3", "": "4"}},
		{name: "HTMLAndSeparators_Escaped", body: map[string]any{"s": "a<b>&c  \u007f"}},
		{name: "ControlCharacters_Escaped", body: map[string]any{"s": "\x00\x01\b\f\t\n\r\"\\/\x1f"}},
		{name: "InvalidUTF8_Replaced", body: map[string]any{"s": string([]byte{0xff, 'x', 0xc3})}},
		{name: "Multibyte_Kept", body: map[string]any{"s": "😀é"}},
		{
			name: "Doubles_ES6Formatting",
			body: map[string]any{
				"one": 1.0, "e21": 1e21, "e20": 1e20, "tiny": 1e-7, "negTiny": -1.5e-7, "frac": 123456789.125,
				"negZero": math.Copysign(0, -1), "max32": 3.4e38, "min": 5e-324,
			},
		},
		{name: "Ints", body: map[string]any{"neg": int64(-9007199254740993), "max": int64(math.MaxInt64)}},
		{name: "Bytes_Base64_EmptyNull", body: map[string]any{"b": []byte{1, 2, 3}, "empty": []byte{}}},
		{
			name: "Nested",
			body: map[string]any{"n": map[string]any{"y": []any{1.5, "q", nil, true, map[string]any{"k": []byte{1}}, []any{}}, "m": map[string]any{}}},
		},
		{name: "NaN_Empty", body: map[string]any{"ok": "x", "nan": math.NaN()}},
		{name: "NestedInf_Empty", body: map[string]any{"x": []any{map[string]any{"inf": math.Inf(-1)}}}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			body := pcommon.NewValueMap()
			require.NoError(t, body.Map().FromRaw(testCase.body))
			stash := pcommon.NewValueEmpty()
			stashOriginalBody(body, stash, nil)
			assert.Equal(t, body.AsString(), stash.Str())
		})
	}
}
