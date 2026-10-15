package utils

import (
	"encoding/json"
	"strings"
)

func MaterializeJSONNumbers(m map[string]any) map[string]any {
	for k, v := range m {
		m[k] = materializeValue(v)
	}
	return m
}

func materializeValue(v any) any {
	switch t := v.(type) {
	case json.Number:
		return materializeNumber(t)
	case map[string]any:
		return MaterializeJSONNumbers(t)
	case []any:
		for i, el := range t {
			t[i] = materializeValue(el)
		}
		return t
	}
	return v
}

func materializeNumber(n json.Number) any {
	if i, err := n.Int64(); err == nil {
		return i
	}
	if strings.ContainsAny(n.String(), ".eE") {
		if f, err := n.Float64(); err == nil {
			return f
		}
	}
	return n.String()
}
