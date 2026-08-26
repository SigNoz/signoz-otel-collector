package utils

// SanitizeJSONFloats replaces NaN/Inf float64 values (recursively, in place) with nil so
// json.Marshal does not reject the whole value.
func SanitizeJSONFloats(v any) any {
	switch val := v.(type) {
	case float64:
		if IsValidFloat(val) {
			return val
		}
		return nil
	case map[string]any:
		for k, vv := range val {
			val[k] = SanitizeJSONFloats(vv)
		}
		return val
	case []any:
		for i, vv := range val {
			val[i] = SanitizeJSONFloats(vv)
		}
		return val
	default:
		return v
	}
}
