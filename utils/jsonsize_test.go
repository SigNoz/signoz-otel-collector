package utils

import (
	encodingjson "encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func strValue(s string) func() pcommon.Value {
	return func() pcommon.Value { return pcommon.NewValueStr(s) }
}

func doubleValue(f float64) func() pcommon.Value {
	return func() pcommon.Value { return pcommon.NewValueDouble(f) }
}

func bytesValue(b []byte) func() pcommon.Value {
	return func() pcommon.Value {
		v := pcommon.NewValueBytes()
		v.Bytes().FromRaw(b)
		return v
	}
}

func mapValue(fill func(m pcommon.Map)) func() pcommon.Value {
	return func() pcommon.Value {
		v := pcommon.NewValueMap()
		fill(v.Map())
		return v
	}
}

// Put replaces existing keys, so duplicates need the OTLP unmarshaler.
func otlpMapBody(t *testing.T, kvs ...string) pcommon.Value {
	raw := `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"body":{"kvlistValue":{"values":[` +
		strings.Join(kvs, ",") + `]}}}]}]}]}`
	logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs([]byte(raw))
	require.NoError(t, err)
	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	require.Equal(t, len(kvs), body.Map().Len())
	return body
}

func duplicateKeyBody(t *testing.T) pcommon.Value {
	return otlpMapBody(t,
		`{"key":"a","value":{"stringValue":"short"}}`,
		`{"key":"b","value":{"intValue":"1"}}`,
		`{"key":"a","value":{"stringValue":"a much longer value"}}`,
	)
}

// setNthDouble sets the nth (0-based) value stored under key, since Get only finds the first.
func setNthDouble(m pcommon.Map, key string, n int, f float64) {
	seen := 0
	m.Range(func(k string, v pcommon.Value) bool {
		if k != key {
			return true
		}
		if seen == n {
			v.SetDouble(f)
			return false
		}
		seen++
		return true
	})
}

func TestJSONSize(t *testing.T) {
	testCases := []struct {
		name  string
		value func() pcommon.Value
		std   string
		goccy string // set only where goccy/go-json differs from std
		ok    bool   // false where Marshal fails
	}{
		{name: "Empty", value: pcommon.NewValueEmpty, std: `null`, ok: true},
		{name: "Str_Plain", value: strValue("message"), std: `"message"`, ok: true},
		{name: "Str_Empty", value: strValue(""), std: `""`, ok: true},
		{name: "Str_QuoteBackslash", value: strValue("\"\\"), std: `"\"\\"`, ok: true},
		{name: "Str_NewlineReturnTab", value: strValue("\n\r\t"), std: `"\n\r\t"`, ok: true},
		{name: "Str_BackspaceFormfeed_GoccyDiffers", value: strValue("\b\f"), std: `"\b\f"`, goccy: `"\u0008\u000c"`, ok: true},
		{name: "Str_Control", value: strValue("\x00\x01\x1f"), std: `"\u0000\u0001\u001f"`, ok: true},
		{name: "Str_Delete", value: strValue("\x7f"), std: "\"\x7f\"", ok: true},
		{name: "Str_HTML", value: strValue("<a>&"), std: `"\u003ca\u003e\u0026"`, ok: true},
		{name: "Str_Slash", value: strValue("a/b"), std: `"a/b"`, ok: true},
		{name: "Str_Multibyte", value: strValue("\xc3\xa9\xe6\x97\xa5\xf0\x9f\x98\x80"), std: "\"\xc3\xa9\xe6\x97\xa5\xf0\x9f\x98\x80\"", ok: true},
		{name: "Str_LineParagraphSeparator", value: strValue("\xe2\x80\xa8\xe2\x80\xa9"), std: `"\u2028\u2029"`, ok: true},
		{name: "Str_InvalidByte", value: strValue("a\xffb"), std: `"a\ufffdb"`, ok: true},
		{name: "Str_TruncatedRune", value: strValue("\xe2\x80"), std: `"\ufffd\ufffd"`, ok: true},
		{name: "Str_ReplacementChar", value: strValue("\xef\xbf\xbd"), std: "\"\xef\xbf\xbd\"", ok: true},
		{name: "Bool_True", value: func() pcommon.Value { return pcommon.NewValueBool(true) }, std: `true`, ok: true},
		{name: "Bool_False", value: func() pcommon.Value { return pcommon.NewValueBool(false) }, std: `false`, ok: true},
		{name: "Int_Zero", value: func() pcommon.Value { return pcommon.NewValueInt(0) }, std: `0`, ok: true},
		{name: "Int_Min", value: func() pcommon.Value { return pcommon.NewValueInt(math.MinInt64) }, std: `-9223372036854775808`, ok: true},
		{name: "Int_Max", value: func() pcommon.Value { return pcommon.NewValueInt(math.MaxInt64) }, std: `9223372036854775807`, ok: true},
		{name: "Double_Zero", value: doubleValue(0), std: `0`, ok: true},
		{name: "Double_NegativeZero", value: doubleValue(math.Copysign(0, -1)), std: `-0`, ok: true},
		{name: "Double_Fraction", value: doubleValue(-123.456), std: `-123.456`, ok: true},
		{name: "Double_SmallestFixed", value: doubleValue(1e-6), std: `0.000001`, ok: true},
		{name: "Double_SingleDigitExponent_GoccyDiffers", value: doubleValue(1e-7), std: `1e-7`, goccy: `1e-07`, ok: true},
		{name: "Double_SingleDigitExponentFraction_GoccyDiffers", value: doubleValue(-2.5e-8), std: `-2.5e-8`, goccy: `-2.5e-08`, ok: true},
		{name: "Double_TwoDigitExponent", value: doubleValue(1.5e-10), std: `1.5e-10`, ok: true},
		{name: "Double_Subnormal", value: doubleValue(5e-324), std: `5e-324`, ok: true},
		{name: "Double_LargestFixed", value: doubleValue(1e20), std: `100000000000000000000`, ok: true},
		{name: "Double_SmallestLargeExponent", value: doubleValue(1e21), std: `1e+21`, ok: true},
		{name: "Double_Max", value: doubleValue(math.MaxFloat64), std: `1.7976931348623157e+308`, ok: true},
		{name: "Double_NaN_Fails", value: doubleValue(math.NaN()), ok: false},
		{name: "Double_PositiveInf_Fails", value: doubleValue(math.Inf(1)), ok: false},
		{name: "Double_NegativeInf_Fails", value: doubleValue(math.Inf(-1)), ok: false},
		{name: "Bytes_Empty", value: bytesValue(nil), std: `null`, ok: true},
		{name: "Bytes_Unpadded", value: bytesValue([]byte{0x00, 0xff, 0x10}), std: `"AP8Q"`, ok: true},
		{name: "Bytes_Padded", value: bytesValue([]byte{0x01}), std: `"AQ=="`, ok: true},
		{name: "Map_Empty", value: pcommon.NewValueMap, std: `{}`, ok: true},
		{name: "Slice_Empty", value: pcommon.NewValueSlice, std: `[]`, ok: true},
		{name: "Map_EscapedKey", value: mapValue(func(m pcommon.Map) { m.PutInt("<k>\n", 1) }), std: `{"\u003ck\u003e\n":1}`, ok: true},
		{name: "Map_Nested_GoccyDiffers", value: mapValue(func(m pcommon.Map) {
			s := m.PutEmptySlice("a")
			s.AppendEmpty().SetInt(1)
			s.AppendEmpty().SetStr("x")
			s.AppendEmpty()
			m.PutEmptyMap("b").PutBool("c", true)
			m.PutDouble("d", 1e-7)
			m.PutStr("e", "\b")
		}), std: `{"a":[1,"x",null],"b":{"c":true},"d":1e-7,"e":"\b"}`, goccy: `{"a":[1,"x",null],"b":{"c":true},"d":1e-07,"e":"\u0008"}`, ok: true},
		{name: "Map_NestedNaN_Fails", value: mapValue(func(m pcommon.Map) {
			m.PutStr("ok", "fine")
			m.PutEmptySlice("list").AppendEmpty().SetDouble(math.NaN())
		}), ok: false},
		{name: "Str_NulInMiddle", value: strValue("a\x00b"), std: `"a\u0000b"`, ok: true},
		{name: "Str_MixedEscapes_GoccyDiffers", value: strValue("<\"\\\n\b>"), std: `"\u003c\"\\\n\b\u003e"`, goccy: `"\u003c\"\\\n\u0008\u003e"`, ok: true},
		{name: "Str_OverlongEncoding", value: strValue("\xc0\xaf"), std: `"\ufffd\ufffd"`, ok: true},
		{name: "Str_SurrogateHalf", value: strValue("\xed\xa0\x80"), std: `"\ufffd\ufffd\ufffd"`, ok: true},
		{name: "Str_MaxRune", value: strValue("\xf4\x8f\xbf\xbf"), std: "\"\xf4\x8f\xbf\xbf\"", ok: true},
		{name: "Str_BeyondMaxRune", value: strValue("\xf4\x90\x80\x80"), std: `"\ufffd\ufffd\ufffd\ufffd"`, ok: true},
		{name: "Str_TruncatedFourByte", value: strValue("\xf0\x9f\x98"), std: `"\ufffd\ufffd\ufffd"`, ok: true},
		{name: "Str_SeparatorNextToInvalid", value: strValue("\xe2\x80\xa8\xff"), std: `"\u2028\ufffd"`, ok: true},
		{name: "Int_NegativeOne", value: func() pcommon.Value { return pcommon.NewValueInt(-1) }, std: `-1`, ok: true},
		{name: "Double_JustBelowSmallCutoff_GoccyDiffers", value: doubleValue(math.Nextafter(1e-6, 0)), std: `9.999999999999997e-7`, goccy: `9.999999999999997e-07`, ok: true},
		{name: "Double_NineExponent_GoccyDiffers", value: doubleValue(1e-9), std: `1e-9`, goccy: `1e-09`, ok: true},
		{name: "Double_ThreeDigitExponent", value: doubleValue(1e-100), std: `1e-100`, ok: true},
		{name: "Double_JustBelowLargeCutoff", value: doubleValue(math.Nextafter(1e21, 0)), std: `999999999999999900000`, ok: true},
		{name: "Double_NegativeLargeExponent", value: doubleValue(-1e21), std: `-1e+21`, ok: true},
		{name: "Double_IntegerValued", value: doubleValue(1e15), std: `1000000000000000`, ok: true},
		{name: "Double_MaxInt64", value: doubleValue(float64(math.MaxInt64)), std: `9223372036854776000`, ok: true},
		{name: "Double_ShortestRoundTrip", value: doubleValue(0.30000000000000004), std: `0.30000000000000004`, ok: true},
		{name: "Bytes_TwoBytes", value: bytesValue([]byte{0x01, 0x02}), std: `"AQI="`, ok: true},
		{name: "Bytes_FourBytes", value: bytesValue([]byte{0x01, 0x02, 0x03, 0x04}), std: `"AQIDBA=="`, ok: true},
		{name: "Map_EmptyBytesValue", value: mapValue(func(m pcommon.Map) { m.PutEmptyBytes("b") }), std: `{"b":null}`, ok: true},
		{name: "Map_EmptyKey", value: mapValue(func(m pcommon.Map) { m.PutInt("", 1) }), std: `{"":1}`, ok: true},
		{name: "Map_EmptyValue", value: mapValue(func(m pcommon.Map) { m.PutEmpty("a") }), std: `{"a":null}`, ok: true},
		{name: "Map_KeyOrder", value: mapValue(func(m pcommon.Map) {
			m.PutInt("z", 1)
			m.PutInt("a", 2)
			m.PutInt("m", 3)
		}), std: `{"a":2,"m":3,"z":1}`, ok: true},
		{name: "Map_NestedInf_Fails", value: mapValue(func(m pcommon.Map) {
			m.PutEmptyMap("a").PutEmptyMap("b").PutDouble("c", math.Inf(-1))
		}), ok: false},
		{name: "Slice_NestedEmpty", value: func() pcommon.Value {
			v := pcommon.NewValueSlice()
			v.Slice().AppendEmpty().SetEmptySlice().AppendEmpty().SetEmptySlice()
			v.Slice().AppendEmpty().SetEmptyMap()
			return v
		}, std: `[[[]],{}]`, ok: true},
		{name: "Slice_Mixed", value: func() pcommon.Value {
			v := pcommon.NewValueSlice()
			v.Slice().AppendEmpty().SetBool(true)
			v.Slice().AppendEmpty().SetInt(-1)
			v.Slice().AppendEmpty().SetStr("x")
			v.Slice().AppendEmpty()
			v.Slice().AppendEmpty().SetEmptyMap().PutEmptySlice("k")
			return v
		}, std: `[true,-1,"x",null,{"k":[]}]`, ok: true},
		{name: "Slice_NaN_Fails", value: func() pcommon.Value {
			v := pcommon.NewValueSlice()
			v.Slice().AppendEmpty().SetInt(1)
			v.Slice().AppendEmpty().SetDouble(math.NaN())
			return v
		}, ok: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value := testCase.value()
			raw := value.AsRaw()

			stdBytes, stdErr := encodingjson.Marshal(raw)
			stdSize, stdOK := JSONSize(value)
			goccyBytes, goccyErr := json.Marshal(raw)
			goccySize, goccyOK := GoccyJSONSize(value)

			if !testCase.ok {
				require.Error(t, stdErr, "encoding/json no longer fails on this value")
				require.Error(t, goccyErr, "goccy/go-json no longer fails on this value")
				assert.False(t, stdOK)
				assert.False(t, goccyOK)
				return
			}

			wantGoccy := testCase.goccy
			if wantGoccy == "" {
				wantGoccy = testCase.std
			}
			require.NoError(t, stdErr)
			require.NoError(t, goccyErr)
			assert.Equal(t, testCase.std, string(stdBytes), "encoding/json output changed")
			assert.Equal(t, wantGoccy, string(goccyBytes), "goccy/go-json output changed")

			assert.True(t, stdOK)
			assert.True(t, goccyOK)
			assert.Equal(t, len(testCase.std), stdSize)
			assert.Equal(t, len(wantGoccy), goccySize)
		})
	}
}

func TestJSONSize_DuplicateKeys(t *testing.T) {
	wideKVs := func() []string {
		kvs := make([]string, 0, 41)
		for i := range 40 {
			kvs = append(kvs, fmt.Sprintf(`{"key":"k%d","value":{"intValue":"%d"}}`, i, i))
		}
		return append(kvs, `{"key":"k0","value":{"stringValue":"last"}}`)
	}

	testCases := []struct {
		name  string
		value func(t *testing.T) pcommon.Value
		want  string // AsRaw keeps the last value per key; empty when Marshal fails
	}{
		{name: "LastWins", value: duplicateKeyBody, want: `{"a":"a much longer value","b":1}`},
		{name: "FirstNaN_LastFinite", value: func(t *testing.T) pcommon.Value {
			v := otlpMapBody(t, `{"key":"a","value":{"doubleValue":1}}`, `{"key":"b","value":{"boolValue":true}}`, `{"key":"a","value":{"stringValue":"x"}}`)
			setNthDouble(v.Map(), "a", 0, math.NaN())
			return v
		}, want: `{"a":"x","b":true}`},
		{name: "LastNaN_Fails", value: func(t *testing.T) pcommon.Value {
			v := otlpMapBody(t, `{"key":"a","value":{"stringValue":"x"}}`, `{"key":"a","value":{"doubleValue":2}}`)
			setNthDouble(v.Map(), "a", 1, math.NaN())
			return v
		}},
		{name: "Over32Keys", value: func(t *testing.T) pcommon.Value { return otlpMapBody(t, wideKVs()...) }},
		{name: "Nested", value: func(t *testing.T) pcommon.Value {
			return otlpMapBody(t,
				`{"key":"x","value":{"kvlistValue":{"values":[{"key":"a","value":{"stringValue":"1"}},{"key":"a","value":{"stringValue":"22"}}]}}}`,
				`{"key":"y","value":{"boolValue":true}}`,
			)
		}, want: `{"x":{"a":"22"},"y":true}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value := testCase.value(t)
			raw := value.AsRaw()
			stdBytes, stdErr := encodingjson.Marshal(raw)
			goccyBytes, goccyErr := json.Marshal(raw)
			if testCase.want != "" {
				assert.Equal(t, testCase.want, string(stdBytes))
			}

			stdSize, stdOK := JSONSize(value)
			goccySize, goccyOK := GoccyJSONSize(value)
			assert.Equal(t, stdErr == nil, stdOK)
			assert.Equal(t, goccyErr == nil, goccyOK)
			assert.Equal(t, len(stdBytes), stdSize)
			assert.Equal(t, len(goccyBytes), goccySize)
			assert.Equal(t, len(value.AsString()), AsStringSize(value))
		})
	}
}

func TestJSONSize_Generated(t *testing.T) {
	type generatedCase struct {
		name  string
		value func() pcommon.Value
	}
	testCases := make([]generatedCase, 0, 262)
	for b := range 256 {
		s := string([]byte{byte(b)})
		testCases = append(testCases, generatedCase{
			name:  fmt.Sprintf("Byte_0x%02x", b),
			value: mapValue(func(m pcommon.Map) { m.PutEmptySlice(s).AppendEmpty().SetStr("a" + s + "b") }),
		})
	}
	testCases = append(testCases,
		generatedCase{name: "Str_OneMiB", value: strValue(strings.Repeat("a<\xc3\xa9\n", 1<<18))},
		generatedCase{name: "Nesting_500Levels", value: func() pcommon.Value {
			v := pcommon.NewValueMap()
			m := v.Map()
			for i := range 500 {
				if i%2 == 0 {
					m = m.PutEmptySlice("s").AppendEmpty().SetEmptyMap()
				} else {
					m = m.PutEmptyMap("m")
				}
			}
			m.PutDouble("leaf", 1e-7)
			return v
		}},
		generatedCase{name: "Map_1000Keys", value: mapValue(func(m pcommon.Map) {
			for i := range 1000 {
				m.PutInt(fmt.Sprintf("key-%d", i), int64(i*i))
			}
		})},
		generatedCase{name: "Map_32Keys", value: mapValue(func(m pcommon.Map) {
			for i := range 32 {
				m.PutStr(fmt.Sprintf("k%d", i), "\b")
			}
		})},
		generatedCase{name: "Map_33Keys", value: mapValue(func(m pcommon.Map) {
			for i := range 33 {
				m.PutStr(fmt.Sprintf("k%d", i), "\f")
			}
		})},
		generatedCase{name: "Slice_1000Doubles", value: func() pcommon.Value {
			v := pcommon.NewValueSlice()
			for i := range 1000 {
				v.Slice().AppendEmpty().SetDouble(float64(i) * 1.37e-9)
			}
			return v
		}},
	)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value := testCase.value()
			raw := value.AsRaw()
			stdBytes, stdErr := encodingjson.Marshal(raw)
			goccyBytes, goccyErr := json.Marshal(raw)
			require.NoError(t, stdErr)
			require.NoError(t, goccyErr)

			stdSize, stdOK := JSONSize(value)
			goccySize, goccyOK := GoccyJSONSize(value)
			assert.True(t, stdOK)
			assert.True(t, goccyOK)
			assert.Equal(t, len(stdBytes), stdSize)
			assert.Equal(t, len(goccyBytes), goccySize)
			assert.Equal(t, len(value.AsString()), AsStringSize(value))
		})
	}
}

func TestAsStringSize(t *testing.T) {
	testCases := []struct {
		name  string
		value func() pcommon.Value
		want  string
	}{
		{name: "Empty", value: pcommon.NewValueEmpty, want: ""},
		{name: "Str_NotEncoded", value: strValue("<a>\n\"\xc3\xa9"), want: "<a>\n\"\xc3\xa9"},
		{name: "Int", value: func() pcommon.Value { return pcommon.NewValueInt(-42) }, want: "-42"},
		{name: "Double", value: doubleValue(1e-7), want: "1e-7"},
		{name: "Double_NaN_ErrorText", value: doubleValue(math.NaN()), want: "json: unsupported value: NaN"},
		{name: "Bool", value: func() pcommon.Value { return pcommon.NewValueBool(false) }, want: "false"},
		{name: "Bytes", value: bytesValue([]byte{0x00, 0xff, 0x10}), want: "AP8Q"},
		{name: "Bytes_Empty", value: bytesValue(nil), want: ""},
		{name: "Map_Empty", value: pcommon.NewValueMap, want: "{}"},
		{name: "Map", value: mapValue(func(m pcommon.Map) {
			m.PutStr("message", "<b>")
			m.PutDouble("d", 1e-7)
		}), want: `{"d":1e-7,"message":"\u003cb\u003e"}`},
		{name: "Map_NaN_Empty", value: mapValue(func(m pcommon.Map) { m.PutDouble("d", math.NaN()) }), want: ""},
		{name: "Slice", value: func() pcommon.Value {
			v := pcommon.NewValueSlice()
			v.Slice().AppendEmpty().SetStr("\b")
			v.Slice().AppendEmpty()
			return v
		}, want: `["\b",null]`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value := testCase.value()
			assert.Equal(t, testCase.want, value.AsString(), "pdata AsString output changed")
			assert.Equal(t, len(testCase.want), AsStringSize(value))
		})
	}
}

func TestJSONMapSize(t *testing.T) {
	newMap := func() pcommon.Map {
		m := pcommon.NewMap()
		m.PutStr("keep", "value")
		m.PutStr("signoz.internal.original_body", "dropped")
		m.PutInt("count", 3)
		return m
	}
	internal := func(k string) bool { return strings.HasPrefix(k, "signoz.internal.") }

	testCases := []struct {
		name string
		m    func() pcommon.Map
		skip func(string) bool
		want string
		ok   bool
	}{
		{name: "Skip_Nil", m: newMap, skip: nil, want: `{"count":3,"keep":"value","signoz.internal.original_body":"dropped"}`, ok: true},
		{name: "Skip_Prefix", m: newMap, skip: internal, want: `{"count":3,"keep":"value"}`, ok: true},
		{name: "Skip_All", m: newMap, skip: func(string) bool { return true }, want: `{}`, ok: true},
		{name: "Skip_NaNKey", m: func() pcommon.Map {
			m := newMap()
			m.PutDouble("signoz.internal.nan", math.NaN())
			return m
		}, skip: internal, want: `{"count":3,"keep":"value"}`, ok: true},
		{name: "Keep_NaNKey_Fails", m: func() pcommon.Map {
			m := newMap()
			m.PutDouble("nan", math.NaN())
			return m
		}, skip: internal, ok: false},
		{name: "DuplicateKeys_Skip", m: func() pcommon.Map { return duplicateKeyBody(t).Map() },
			skip: func(k string) bool { return k == "b" }, want: `{"a":"a much longer value"}`, ok: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := testCase.m()
			size, ok := JSONMapSize(m, testCase.skip)
			goccySize, goccyOK := GoccyJSONMapSize(m, testCase.skip)
			assert.Equal(t, testCase.ok, ok)
			assert.Equal(t, testCase.ok, goccyOK)
			if testCase.ok {
				assert.Equal(t, len(testCase.want), size)
				assert.Equal(t, len(testCase.want), goccySize)
			}
		})
	}
}

func randomString(r *rand.Rand) string {
	parts := []string{
		"a", "Z", "0", " ", "message", `"`, `\`, "/", "\b", "\f", "\n", "\r", "\t", "\x00", "\x01", "\x1f", "\x7f",
		"<", ">", "&", "\xc3\xa9", "\xe6\x97\xa5", "\xf0\x9f\x98\x80", "\xe2\x80\xa8", "\xe2\x80\xa9", "\xff", "\xe2\x80", "\xc3", "\xef\xbf\xbd",
	}
	var sb strings.Builder
	for range r.IntN(12) {
		sb.WriteString(parts[r.IntN(len(parts))])
	}
	return sb.String()
}

func randomDouble(r *rand.Rand, allowNaN bool) float64 {
	if allowNaN && r.IntN(50) == 0 {
		return math.NaN()
	}
	if r.IntN(3) == 0 {
		return r.NormFloat64() * math.Pow(10, float64(r.IntN(50)-25))
	}
	edges := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.1, 123.456, 1e-6, 9.99e-7, 1e-7, 1.5e-10, 1e20, 999999999999999999999, 1e21, 1.5e21,
		-2.5e-8, 1e100, -math.MaxFloat64, 5e-324, 1 << 53, 1<<63 - 1,
	}
	return edges[r.IntN(len(edges))]
}

func fillRandomValue(r *rand.Rand, v pcommon.Value, depth int, allowNaN bool) {
	kinds := 8
	if depth > 3 {
		kinds = 6 // no more nesting
	}
	switch r.IntN(kinds) {
	case 0:
		v.SetStr(randomString(r))
	case 1:
		v.SetInt(r.Int64() - math.MaxInt64/2)
	case 2:
		v.SetDouble(randomDouble(r, allowNaN))
	case 3:
		v.SetBool(r.IntN(2) == 0)
	case 4:
		b := make([]byte, r.IntN(8))
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		v.SetEmptyBytes().FromRaw(b)
	case 5:
		// left empty
	case 6:
		m := v.SetEmptyMap()
		for range r.IntN(6) {
			fillRandomValue(r, m.PutEmpty(randomString(r)), depth+1, allowNaN)
		}
	case 7:
		s := v.SetEmptySlice()
		for range r.IntN(6) {
			fillRandomValue(r, s.AppendEmpty(), depth+1, allowNaN)
		}
	}
}

func TestJSONSize_RandomValues_MatchEncoders(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 20000 {
		body := pcommon.NewValueEmpty()
		switch i % 3 {
		case 0:
			m := body.SetEmptyMap()
			for range r.IntN(8) {
				fillRandomValue(r, m.PutEmpty(randomString(r)), 1, i%2 == 0)
			}
		case 1:
			s := body.SetEmptySlice()
			for range r.IntN(8) {
				fillRandomValue(r, s.AppendEmpty(), 1, i%2 == 0)
			}
		default:
			fillRandomValue(r, body, 0, i%2 == 0)
		}

		raw := body.AsRaw()
		stdBytes, stdErr := encodingjson.Marshal(raw)
		stdSize, stdOK := JSONSize(body)
		goccyBytes, goccyErr := json.Marshal(raw)
		goccySize, goccyOK := GoccyJSONSize(body)

		ok := assert.Equalf(t, len(body.AsString()), AsStringSize(body), "body %q", body.AsString()) &&
			assert.Equalf(t, stdErr == nil, stdOK, "body %q", body.AsString()) &&
			assert.Equalf(t, goccyErr == nil, goccyOK, "goccy, body %q", body.AsString()) &&
			(!stdOK || assert.Equalf(t, len(stdBytes), stdSize, "body %q", body.AsString())) &&
			(!goccyOK || assert.Equalf(t, len(goccyBytes), goccySize, "goccy, body %q", body.AsString()))
		if !ok {
			return
		}
	}
}

func TestJSONSize_ZeroAllocs(t *testing.T) {
	body := pcommon.NewValueMap()
	body.Map().PutStr("message", strings.Repeat("GET /api/v1/orders?id=<12345> 200 & done ", 10))
	body.Map().PutDouble("latency", 1.5e-7)
	nested := body.Map().PutEmptyMap("request")
	nested.PutInt("status", 200)
	nested.PutEmptySlice("tags").AppendEmpty().SetStr("\xe6\x97\xa5")

	allocs := testing.AllocsPerRun(100, func() {
		_ = AsStringSize(body)
		_, _ = GoccyJSONSize(body)
	})
	assert.Zero(t, allocs)
}

func FuzzJSONSize(f *testing.F) {
	for _, s := range []string{"message", "\b\f", "<a>&", "\xe2\x80\xa8", "\xff", "\xe2\x80"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		body := pcommon.NewValueMap()
		body.Map().PutStr(s, s)
		body.Map().PutEmptySlice("list").AppendEmpty().SetStr(s)
		assert.Equal(t, len(body.AsString()), AsStringSize(body))

		goccyBytes, err := json.Marshal(body.AsRaw())
		require.NoError(t, err)
		goccySize, ok := GoccyJSONSize(body)
		assert.True(t, ok)
		assert.Equal(t, len(goccyBytes), goccySize)
	})
}
