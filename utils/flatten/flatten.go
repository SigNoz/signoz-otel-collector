package flatten

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

// ConvertJSON converts JSON with dotted keys into nested structures.
func ConvertJSON(input string) (string, error) {
	var parsed interface{}

	// Basic validation - check if it looks like JSON
	if len(input) == 0 || !strings.HasPrefix(strings.TrimSpace(input), "{") {
		return "", fmt.Errorf("input is not a valid JSON object")
	}

	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	transformed := nestDottedKeys(parsed)

	result, err := json.Marshal(transformed)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(result), nil
}

// nestDottedKeys converts dotted keys into nested structures efficiently.
func nestDottedKeys(input interface{}) interface{} {
	switch val := input.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})

		// First, process all non-dotted keys to establish the base structure
		for key, value := range val {
			if !strings.Contains(key, ".") {
				result[key] = nestDottedKeys(value)
			}
		}

		// Then, process all dotted keys to merge into the existing structure
		for key, value := range val {
			if strings.Contains(key, ".") {
				nestedValue := nestDottedKeys(value)
				insertNestedKey(result, key, nestedValue)
			}
		}

		return result

	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = nestDottedKeys(v)
		}
		return result

	default:
		return input
	}
}

// insertNestedKey efficiently inserts a value at a dotted key path.
func insertNestedKey(dest map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := dest

	// Navigate to the parent of the final key
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		// Get or create the next level map
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				// If the value exists but isn't a map, replace it with a map
				current[part] = map[string]interface{}{}
				current = current[part].(map[string]interface{})
			}
		} else {
			// Create new map for this level
			current[part] = map[string]interface{}{}
			current = current[part].(map[string]interface{})
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}
