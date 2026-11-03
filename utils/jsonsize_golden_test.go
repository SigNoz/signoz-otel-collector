package utils

import (
	encodingjson "encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// The golden tests pin the exact bytes each encoder writes, not just that the helpers agree with them,
// so a Go or goccy/go-json upgrade that changes the encoding fails here instead of silently changing metered sizes.

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

func duplicateKeyBody(t *testing.T) pcommon.Value {
	// pcommon.Map's Put methods replace existing keys, so build the duplicates through the OTLP JSON unmarshaler
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

func TestJSONSizeGolden(t *testing.T) {
	testCases := []struct {
		name  string
		value func() pcommon.Value
		// std is encoding/json's output, goccy is goccy/go-json's when it differs; empty with ok false means Marshal fails
		std   string
		goccy string
		ok    bool
	}{
		{name: "Empty", value: pcommon.NewValueEmpty, std: `null`, ok: true},
		{name: "StrPlain", value: strValue("message"), std: `"message"`, ok: true},
		{name: "StrEmpty", value: strValue(""), std: `""`, ok: true},
		{name: "StrQuoteBackslash", value: strValue("\"\\"), std: `"\"\\"`, ok: true},
		{name: "StrNewlineReturnTab", value: strValue("\n\r\t"), std: `"\n\r\t"`, ok: true},
		{name: "StrBackspaceFormfeed", value: strValue("\b\f"), std: `"\b\f"`, goccy: `"\u0008\u000c"`, ok: true},
		{name: "StrControl", value: strValue("\x00\x01\x1f"), std: `"\u0000\u0001\u001f"`, ok: true},
		{name: "StrDelete", value: strValue("\x7f"), std: "\"\x7f\"", ok: true},
		{name: "StrHTML", value: strValue("<a>&"), std: `"\u003ca\u003e\u0026"`, ok: true},
		{name: "StrSlash", value: strValue("a/b"), std: `"a/b"`, ok: true},
		{name: "StrMultibyte", value: strValue("\xc3\xa9\xe6\x97\xa5\xf0\x9f\x98\x80"), std: "\"\xc3\xa9\xe6\x97\xa5\xf0\x9f\x98\x80\"", ok: true},
		{name: "StrLineParagraphSeparator", value: strValue("\xe2\x80\xa8\xe2\x80\xa9"), std: `"\u2028\u2029"`, ok: true},
		{name: "StrInvalidByte", value: strValue("a\xffb"), std: `"a\ufffdb"`, ok: true},
		{name: "StrTruncatedRune", value: strValue("\xe2\x80"), std: `"\ufffd\ufffd"`, ok: true},
		{name: "StrReplacementChar", value: strValue("\xef\xbf\xbd"), std: "\"\xef\xbf\xbd\"", ok: true},
		{name: "BoolTrue", value: func() pcommon.Value { return pcommon.NewValueBool(true) }, std: `true`, ok: true},
		{name: "BoolFalse", value: func() pcommon.Value { return pcommon.NewValueBool(false) }, std: `false`, ok: true},
		{name: "IntZero", value: func() pcommon.Value { return pcommon.NewValueInt(0) }, std: `0`, ok: true},
		{name: "IntMin", value: func() pcommon.Value { return pcommon.NewValueInt(math.MinInt64) }, std: `-9223372036854775808`, ok: true},
		{name: "IntMax", value: func() pcommon.Value { return pcommon.NewValueInt(math.MaxInt64) }, std: `9223372036854775807`, ok: true},
		{name: "DoubleZero", value: doubleValue(0), std: `0`, ok: true},
		{name: "DoubleNegativeZero", value: doubleValue(math.Copysign(0, -1)), std: `-0`, ok: true},
		{name: "DoubleFraction", value: doubleValue(-123.456), std: `-123.456`, ok: true},
		{name: "DoubleSmallestFixed", value: doubleValue(1e-6), std: `0.000001`, ok: true},
		{name: "DoubleSingleDigitExponent", value: doubleValue(1e-7), std: `1e-7`, goccy: `1e-07`, ok: true},
		{name: "DoubleSingleDigitExponentFraction", value: doubleValue(-2.5e-8), std: `-2.5e-8`, goccy: `-2.5e-08`, ok: true},
		{name: "DoubleTwoDigitExponent", value: doubleValue(1.5e-10), std: `1.5e-10`, ok: true},
		{name: "DoubleSubnormal", value: doubleValue(5e-324), std: `5e-324`, ok: true},
		{name: "DoubleLargestFixed", value: doubleValue(1e20), std: `100000000000000000000`, ok: true},
		{name: "DoubleSmallestLargeExponent", value: doubleValue(1e21), std: `1e+21`, ok: true},
		{name: "DoubleMax", value: doubleValue(math.MaxFloat64), std: `1.7976931348623157e+308`, ok: true},
		{name: "DoubleNaN", value: doubleValue(math.NaN()), ok: false},
		{name: "DoublePositiveInf", value: doubleValue(math.Inf(1)), ok: false},
		{name: "DoubleNegativeInf", value: doubleValue(math.Inf(-1)), ok: false},
		{name: "BytesEmpty", value: bytesValue(nil), std: `null`, ok: true},
		{name: "Bytes", value: bytesValue([]byte{0x00, 0xff, 0x10}), std: `"AP8Q"`, ok: true},
		{name: "BytesPadded", value: bytesValue([]byte{0x01}), std: `"AQ=="`, ok: true},
		{name: "MapEmpty", value: pcommon.NewValueMap, std: `{}`, ok: true},
		{name: "SliceEmpty", value: pcommon.NewValueSlice, std: `[]`, ok: true},
		{name: "MapEscapedKey", value: mapValue(func(m pcommon.Map) { m.PutInt("<k>\n", 1) }), std: `{"\u003ck\u003e\n":1}`, ok: true},
		{name: "MapNested", value: mapValue(func(m pcommon.Map) {
			s := m.PutEmptySlice("a")
			s.AppendEmpty().SetInt(1)
			s.AppendEmpty().SetStr("x")
			s.AppendEmpty()
			m.PutEmptyMap("b").PutBool("c", true)
			m.PutDouble("d", 1e-7)
			m.PutStr("e", "\b")
		}), std: `{"a":[1,"x",null],"b":{"c":true},"d":1e-7,"e":"\b"}`, goccy: `{"a":[1,"x",null],"b":{"c":true},"d":1e-07,"e":"\u0008"}`, ok: true},
		{name: "MapNestedNaN", value: mapValue(func(m pcommon.Map) {
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
			require.Equal(t, testCase.std, string(stdBytes), "encoding/json output changed")
			require.Equal(t, wantGoccy, string(goccyBytes), "goccy/go-json output changed")

			assert.True(t, stdOK)
			assert.True(t, goccyOK)
			assert.Equal(t, len(testCase.std), stdSize)
			assert.Equal(t, len(wantGoccy), goccySize)
		})
	}
}

func TestJSONSizeGoldenDuplicateKeys(t *testing.T) {
	// AsRaw keeps the last value for a repeated key
	const want = `{"a":"a much longer value","b":1}`
	body := duplicateKeyBody(t)
	require.True(t, hasDuplicateKeys(body.Map()))

	stdBytes, err := encodingjson.Marshal(body.AsRaw())
	require.NoError(t, err)
	require.Equal(t, want, string(stdBytes))

	size, ok := JSONSize(body)
	assert.True(t, ok)
	assert.Equal(t, len(want), size)

	size, ok = GoccyJSONSize(body)
	assert.True(t, ok)
	assert.Equal(t, len(want), size)

	assert.Equal(t, len(want), AsStringSize(body))
}

func TestAsStringSizeGolden(t *testing.T) {
	testCases := []struct {
		name  string
		value func() pcommon.Value
		want  string // body.AsString()
	}{
		{name: "Empty", value: pcommon.NewValueEmpty, want: ""},
		// top-level strings are not JSON encoded
		{name: "Str", value: strValue("<a>\n\"\xc3\xa9"), want: "<a>\n\"\xc3\xa9"},
		{name: "Int", value: func() pcommon.Value { return pcommon.NewValueInt(-42) }, want: "-42"},
		{name: "Double", value: doubleValue(1e-7), want: "1e-7"},
		// pdata writes the encoding/json error for a top-level NaN, and that text is what gets metered
		{name: "DoubleNaN", value: doubleValue(math.NaN()), want: "json: unsupported value: NaN"},
		{name: "Bool", value: func() pcommon.Value { return pcommon.NewValueBool(false) }, want: "false"},
		{name: "Bytes", value: bytesValue([]byte{0x00, 0xff, 0x10}), want: "AP8Q"},
		{name: "BytesEmpty", value: bytesValue(nil), want: ""},
		{name: "MapEmpty", value: pcommon.NewValueMap, want: "{}"},
		{name: "Map", value: mapValue(func(m pcommon.Map) {
			m.PutStr("message", "<b>")
			m.PutDouble("d", 1e-7)
		}), want: `{"d":1e-7,"message":"\u003cb\u003e"}`},
		// AsString drops the error from Marshal and returns ""
		{name: "MapNaN", value: mapValue(func(m pcommon.Map) { m.PutDouble("d", math.NaN()) }), want: ""},
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
			require.Equal(t, testCase.want, value.AsString(), "pdata AsString output changed")
			assert.Equal(t, len(testCase.want), AsStringSize(value))
		})
	}
}

func TestJSONMapSizeGoldenSkip(t *testing.T) {
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
		{name: "NilSkip", m: newMap, skip: nil, want: `{"count":3,"keep":"value","signoz.internal.original_body":"dropped"}`, ok: true},
		{name: "SkipPrefix", m: newMap, skip: internal, want: `{"count":3,"keep":"value"}`, ok: true},
		{name: "SkipAll", m: newMap, skip: func(string) bool { return true }, want: `{}`, ok: true},
		{name: "SkipNaN", m: func() pcommon.Map {
			m := newMap()
			m.PutDouble("signoz.internal.nan", math.NaN())
			return m
		}, skip: internal, want: `{"count":3,"keep":"value"}`, ok: true},
		{name: "KeepNaN", m: func() pcommon.Map {
			m := newMap()
			m.PutDouble("nan", math.NaN())
			return m
		}, skip: internal, ok: false},
		{name: "DuplicateKeysSkip", m: func() pcommon.Map { return duplicateKeyBody(t).Map() },
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
