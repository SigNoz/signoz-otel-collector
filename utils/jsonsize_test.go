package utils

import (
	encodingjson "encoding/json"
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
func duplicateKeyBody(t *testing.T) pcommon.Value {
	raw := `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"body":{"kvlistValue":{"values":[` +
		`{"key":"a","value":{"stringValue":"short"}},` +
		`{"key":"b","value":{"intValue":"1"}},` +
		`{"key":"a","value":{"stringValue":"a much longer value"}}` +
		`]}}}]}]}]}`
	logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs([]byte(raw))
	require.NoError(t, err)
	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	require.Equal(t, 3, body.Map().Len())
	return body
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
	body := duplicateKeyBody(t)
	require.True(t, hasDuplicateKeys(body.Map()))

	// AsRaw keeps the last value per key
	want := `{"a":"a much longer value","b":1}`
	stdBytes, err := encodingjson.Marshal(body.AsRaw())
	require.NoError(t, err)
	assert.Equal(t, want, string(stdBytes))

	size, ok := JSONSize(body)
	assert.True(t, ok)
	assert.Equal(t, len(want), size)

	size, ok = GoccyJSONSize(body)
	assert.True(t, ok)
	assert.Equal(t, len(want), size)

	assert.Equal(t, len(want), AsStringSize(body))
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
