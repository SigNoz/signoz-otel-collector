package clickhouselogsexporter

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var bodySizeStringParts = []string{
	"a", "Z", "0", " ", "message", `"`, `\`, "/", "\b", "\f", "\n", "\r", "\t", "\x00", "\x01", "\x1f", "\x7f",
	"<", ">", "&", "\u00e9", "\u65e5\u672c", "\U0001F600", "\u2028", "\u2029", "\xff", "\xe2\x80", "\xc3", "\ufffd",
}

var bodySizeFloats = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.1, 123.456, 1e-6, 9.99e-7, 1e-7, 1.5e-10, 1e20, 999999999999999999999, 1e21, 1.5e21,
	-2.5e-8, 1e100, -1.7976931348623157e308, 5e-324, 1 << 53, 1<<63 - 1,
}

func randomBodySizeString(r *rand.Rand) string {
	var sb strings.Builder
	for range r.IntN(12) {
		sb.WriteString(bodySizeStringParts[r.IntN(len(bodySizeStringParts))])
	}
	return sb.String()
}

func fillRandomValue(r *rand.Rand, v pcommon.Value, depth int, allowNaN bool) {
	kinds := 8
	if depth > 3 {
		kinds = 6 // no more nesting
	}
	switch r.IntN(kinds) {
	case 0:
		v.SetStr(randomBodySizeString(r))
	case 1:
		v.SetInt(r.Int64() - math.MaxInt64/2)
	case 2:
		f := bodySizeFloats[r.IntN(len(bodySizeFloats))]
		if r.IntN(3) == 0 {
			f = r.NormFloat64() * math.Pow(10, float64(r.IntN(50)-25))
		}
		if allowNaN && r.IntN(50) == 0 {
			f = math.NaN()
		}
		v.SetDouble(f)
	case 3:
		v.SetBool(r.IntN(2) == 0)
	case 4:
		b := make([]byte, r.IntN(8))
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		v.SetEmptyBytes().FromRaw(b)
	case 5:
		// leave empty, encodes as null
	case 6:
		m := v.SetEmptyMap()
		for range r.IntN(6) {
			fillRandomValue(r, m.PutEmpty(randomBodySizeString(r)), depth+1, allowNaN)
		}
	case 7:
		s := v.SetEmptySlice()
		for range r.IntN(6) {
			fillRandomValue(r, s.AppendEmpty(), depth+1, allowNaN)
		}
	}
}

func TestBodySizeRandomValues(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 20000 {
		body := pcommon.NewValueEmpty()
		switch i % 3 {
		case 0:
			m := body.SetEmptyMap()
			for range r.IntN(8) {
				fillRandomValue(r, m.PutEmpty(randomBodySizeString(r)), 1, i%2 == 0)
			}
		case 1:
			s := body.SetEmptySlice()
			for range r.IntN(8) {
				fillRandomValue(r, s.AppendEmpty(), 1, i%2 == 0)
			}
		default:
			fillRandomValue(r, body, 0, i%2 == 0)
		}

		want := len(body.AsString())
		if got := bodySize(body); got != want {
			require.Equalf(t, want, got, "body %q", body.AsString())
		}
	}
}

func TestBodySizeDuplicateKeys(t *testing.T) {
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
	require.True(t, hasDuplicateKeys(body.Map()))

	assert.Equal(t, len(body.AsString()), bodySize(body))
}

func TestBodySizeDoesNotAllocate(t *testing.T) {
	body := pcommon.NewValueMap()
	body.Map().PutStr("message", strings.Repeat("GET /api/v1/orders?id=<12345> 200 & done ", 10))
	body.Map().PutDouble("latency", 1.5e-7)
	nested := body.Map().PutEmptyMap("request")
	nested.PutInt("status", 200)
	nested.PutEmptySlice("tags").AppendEmpty().SetStr("\u65e5\u672c")

	allocs := testing.AllocsPerRun(100, func() { _ = bodySize(body) })
	assert.Zero(t, allocs)
}

func FuzzBodySizeString(f *testing.F) {
	for _, s := range bodySizeStringParts {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		body := pcommon.NewValueMap()
		body.Map().PutStr(s, s)
		body.Map().PutEmptySlice("list").AppendEmpty().SetStr(s)
		assert.Equal(t, len(body.AsString()), bodySize(body))
	})
}
