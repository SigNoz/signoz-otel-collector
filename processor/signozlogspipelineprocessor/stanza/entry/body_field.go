// brought in as-is from opentelemetry-collector-contrib with
// changes to Get() to enable reading fields inside JSON body directly
package signozstanzaentry

import (
	"fmt"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// BodyField is a field found on an entry body.
type BodyField struct {
	Keys []string
}

// NewBodyField creates a new field from an ordered array of keys.
func NewBodyField(keys ...string) Field {
	if keys == nil {
		keys = []string{}
	}
	return Field{BodyField{
		Keys: keys,
	}}
}

// Parent returns the parent of the current field.
// In the case that the body field points to the root node, it is a no-op.
func (f BodyField) Parent() BodyField {
	if f.isRoot() {
		return f
	}

	keys := f.Keys[:len(f.Keys)-1]
	return BodyField{keys}
}

// Child returns a child of the current field using the given key.
func (f BodyField) Child(key string) BodyField {
	child := make([]string, len(f.Keys), len(f.Keys)+1)
	copy(child, f.Keys)
	child = append(child, key)
	return BodyField{child}
}

// IsRoot returns a boolean indicating if this is a root level field.
func (f BodyField) isRoot() bool {
	return len(f.Keys) == 0
}

// String returns the string representation of this field.
func (f BodyField) String() string {
	return toJSONDot(entry.BodyPrefix, f.Keys)
}

// returns nil if the body was not JSON
func ParseBodyJson(entry *entry.Entry) map[string]any {
	// If body is already a map, return it as-is
	bodyMap, ok := entry.Body.(map[string]any)
	if ok {
		return bodyMap
	}

	// If body JSON has already been parsed and cached, return it
	cachedBodyJsonKey := InternalTempAttributePrefix + "parsedBodyJson"
	if entry.Attributes == nil {
		entry.Attributes = map[string]any{}
	}
	cachedParsedBody, exists := entry.Attributes[cachedBodyJsonKey]
	if exists {
		return cachedParsedBody.(map[string]any)
	}

	// try to parse body as JSON. If successful cache it and return it
	bodyBytes, hasBytes := entry.Body.([]byte)
	if !hasBytes {
		bodyStr, isStr := entry.Body.(string)
		if isStr {
			bodyBytes = []byte(bodyStr)
			hasBytes = true
		}
	}

	if hasBytes {
		var parsedBody map[string]any
		err := json.Unmarshal(bodyBytes, &parsedBody)
		if err == nil {
			// Attributes keys starting with InternalTempAttributePrefix
			// get ignored when entries are converted back to plog.Logs
			// See `convertEntriesToPlogs` in signozlogspipelineprocess/utils.go
			entry.Attributes[cachedBodyJsonKey] = parsedBody
			return parsedBody
		}
	}

	return nil
}

// Get will retrieve a value from an entry's body using the field.
// It will return the value and whether the field existed.
func (f BodyField) Get(entry *entry.Entry) (any, bool) {
	var currentValue = entry.Body

	// If path inside body field has been specified, try to
	// interpret body as a map[string]any - parsing JSON if needed
	if len(f.Keys) > 0 {
		bodyJson := ParseBodyJson(entry)
		if bodyJson != nil {
			currentValue = bodyJson
		}
	}

	for _, key := range f.Keys {
		currentMap, ok := currentValue.(map[string]any)
		if !ok {
			return nil, false
		}

		currentValue, ok = currentMap[key]
		if !ok {
			return nil, false
		}
	}

	return currentValue, true
}

// Set will set a value on an entry's body using the field.
// If a key already exists, it will be overwritten.
func (f BodyField) Set(entry *entry.Entry, value any) error {
	mapValue, isMapValue := value.(map[string]any)
	if isMapValue {
		f.Merge(entry, mapValue)
		return nil
	}

	if f.isRoot() {
		entry.Body = value
		return nil
	}

	currentMap, ok := entry.Body.(map[string]any)
	if !ok {
		currentMap = map[string]any{}
		entry.Body = currentMap
	}

	for i, key := range f.Keys {
		if i == len(f.Keys)-1 {
			currentMap[key] = value
			break
		}
		currentMap = getNestedMap(currentMap, key)
	}
	return nil
}

// Merge will attempt to merge the contents of a map into an entry's body.
// It will overwrite any intermediate values as necessary.
func (f BodyField) Merge(entry *entry.Entry, mapValues map[string]any) {
	currentMap, ok := entry.Body.(map[string]any)
	if !ok {
		currentMap = map[string]any{}
		entry.Body = currentMap
	}

	for _, key := range f.Keys {
		currentMap = getNestedMap(currentMap, key)
	}

	for key, value := range mapValues {
		currentMap[key] = value
	}
}

// Delete removes a value from an entry's body using the field.
// It will return the deleted value and whether the field existed.
func (f BodyField) Delete(entry *entry.Entry) (any, bool) {
	if f.isRoot() {
		oldBody := entry.Body
		entry.Body = nil
		return oldBody, true
	}

	currentValue := entry.Body
	for i, key := range f.Keys {
		currentMap, ok := currentValue.(map[string]any)
		if !ok {
			break
		}

		currentValue, ok = currentMap[key]
		if !ok {
			break
		}

		if i == len(f.Keys)-1 {
			delete(currentMap, key)
			return currentValue, true
		}
	}

	return nil, false
}

/****************
  Serialization
****************/

// UnmarshalJSON will attempt to unmarshal the field from JSON.
func (f *BodyField) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != entry.BodyPrefix {
		return fmt.Errorf("must start with 'body': %s", value)
	}

	*f = BodyField{keys[1:]}
	return nil
}

// UnmarshalYAML will attempt to unmarshal a field from YAML.
func (f *BodyField) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return fmt.Errorf("the field is not a string: %w", err)
	}

	keys, err := fromJSONDot(value)
	if err != nil {
		return err
	}

	if keys[0] != entry.BodyPrefix {
		return fmt.Errorf("must start with 'body': %s", value)
	}

	*f = BodyField{keys[1:]}
	return nil
}

// UnmarshalText will unmarshal a field from text
func (f *BodyField) UnmarshalText(text []byte) error {
	keys, err := fromJSONDot(string(text))
	if err != nil {
		return err
	}

	if keys[0] != entry.BodyPrefix {
		return fmt.Errorf("must start with 'body': %s", text)
	}

	*f = BodyField{keys[1:]}
	return nil
}
