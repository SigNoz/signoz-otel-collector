package internal

import (
	"sort"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/utils/gofuzz"
)

func NewLabelsAsJSONString(name string, ms ...map[string]string) string {
	attrs := make(map[string]string)
	for _, m := range ms {
		for k, v := range m {
			attrs[k] = v
		}
	}

	attrs["__name__"] = name
	return string(marshalLabels(attrs, make([]byte, 0, 128)))
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
