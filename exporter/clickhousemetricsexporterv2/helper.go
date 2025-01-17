package clickhousemetricsexporterv2

import (
	"sort"
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/utils/gofuzz"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func getAllLabels(attrs pcommon.Map, scopeAttrs pcommon.Map, resAttrs pcommon.Map, name string) map[string]string {
	labels := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})

	scopeAttrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})

	resAttrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
	labels["__name__"] = name
	return labels
}

func getJSONString(attrs map[string]string) string {
	return string(marshalLabels(attrs, make([]byte, 0, 128)))
}

func getAttrMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})
	return attrMap
}

// marshalLabels marshals attributes into JSON, appending it to b.
// It preserves an order. It is also significantly faster then json.Marshal.
// It is compatible with ClickHouse JSON functions: https://clickhouse.yandex/docs/en/functions/json_functions.html
func marshalLabels(attrs map[string]string, b []byte) []byte {
	// make sure that the attrs are sorted
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	if len(attrs) == 0 {
		return append(b, '{', '}')
	}

	b = append(b, '{')
	for _, name := range keys {
		value := attrs[name]
		// add label name which can't contain runes that should be escaped
		b = append(b, '"')
		b = append(b, name...)
		b = append(b, '"', ':', '"')

		// add label value while escaping some runes
		for _, c := range []byte(value) {
			switch c {
			case '\\', '"':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, c)
			}
		}

		b = append(b, '"', ',')
	}
	b[len(b)-1] = '}' // replace last comma

	gofuzz.AddToCorpus("json", b)
	return b
}

func makeCacheKey(a, b uint64) string {
	// Pre-allocate a builder with an estimated capacity
	var builder strings.Builder
	builder.Grow(40) // Max length: 20 digits for each uint64 + 1 for the colon

	// Convert and write the first uint64
	builder.WriteString(strconv.FormatUint(a, 10))

	// Write the separator
	builder.WriteByte(':')

	// Convert and write the second uint64
	builder.WriteString(strconv.FormatUint(b, 10))

	return builder.String()
}
