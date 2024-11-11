package flatten

import (
	"fmt"
)

// FlattenJSON flattens a nested JSON into a map[string]string.
func FlattenJSON(data map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch value := value.(type) {
		case map[string]interface{}:
			FlattenJSON(value, fullKey, result)
		case []interface{}:
			for i, v := range value {
				FlattenJSON(map[string]interface{}{fmt.Sprintf("%d", i): v}, fullKey, result)
			}
		default:
			// Convert the value to a string and add to the result map.
			result[fullKey] = fmt.Sprintf("%v", value)
		}
	}
}
