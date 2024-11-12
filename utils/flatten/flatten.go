package flatten

import (
	"fmt"
	"reflect"
)

// FlattenJSON flattens a nested JSON into a map[string]string.
func FlattenJSON(data map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch value := value.(type) {
		case map[string]interface{}:
			subResult := FlattenJSON(value, fullKey)
			for k, v := range subResult {
				result[k] = v
			}
		case []interface{}:
			for i, v := range value {
				subResult := FlattenJSON(map[string]interface{}{fmt.Sprintf("%d", i): v}, fullKey)
				for k, v := range subResult {
					result[k] = v
				}
			}
		case int, int32, int64:
			v := reflect.ValueOf(value)
			result[fullKey] = float64(v.Int())
		case float32:
			result[fullKey] = float64(value)
		case float64, bool:
			result[fullKey] = value
		default:
			result[fullKey] = fmt.Sprintf("%v", value)
		}
	}
	return result
}
