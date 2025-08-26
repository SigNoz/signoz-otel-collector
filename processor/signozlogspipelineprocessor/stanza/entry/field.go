// brought in as-is from opentelemetry-collector-contrib
// with changes to use the customized BodyField copy in this package
package signozstanzaentry

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Field represents a potential field on an entry.
// It is used to get, set, and delete values at this field.
// It is deserialized from JSON dot notation.
type Field struct {
	entry.FieldInterface
}

// RootableField is a Field that may refer directly to "attributes" or "resource"
type RootableField struct {
	Field
}

// UnmarshalJSON will unmarshal a field from JSON
func (f *Field) UnmarshalJSON(raw []byte) error {
	var s string
	err := json.Unmarshal(raw, &s)
	if err != nil {
		return err
	}
	*f, err = NewField(s)
	return err
}

// UnmarshalJSON will unmarshal a field from JSON
func (r *RootableField) UnmarshalJSON(raw []byte) error {
	var s string
	err := json.Unmarshal(raw, &s)
	if err != nil {
		return err
	}
	field, err := newField(s, true)
	*r = RootableField{Field: field}
	return err
}

// UnmarshalYAML will unmarshal a field from YAML
func (f *Field) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	*f, err = NewField(s)
	return err
}

// UnmarshalYAML will unmarshal a field from YAML
func (r *RootableField) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	field, err := newField(s, true)
	*r = RootableField{Field: field}
	return err
}

// UnmarshalText will unmarshal a field from text
func (f *Field) UnmarshalText(text []byte) error {
	field, err := NewField(string(text))
	*f = field
	return err
}

// UnmarshalText will unmarshal a field from text
func (r *RootableField) UnmarshalText(text []byte) error {
	field, err := newField(string(text), true)
	*r = RootableField{Field: field}
	return err
}

func NewField(s string) (Field, error) {
	return newField(s, false)
}

func newField(s string, rootable bool) (Field, error) {
	keys, err := fromJSONDot(s)
	if err != nil {
		return Field{}, fmt.Errorf("splitting field: %w", err)
	}

	switch keys[0] {
	case entry.AttributesPrefix:
		if !rootable && len(keys) == 1 {
			return Field{}, fmt.Errorf("attributes cannot be referenced without subfield")
		}
		return Field{entry.NewAttributeField(keys[1:]...)}, nil
	case entry.ResourcePrefix:
		if !rootable && len(keys) == 1 {
			return Field{}, fmt.Errorf("resource cannot be referenced without subfield")
		}
		return Field{entry.NewResourceField(keys[1:]...)}, nil
	case entry.BodyPrefix:
		return Field{NewBodyField(keys[1:]...)}, nil
	default:
		return Field{}, fmt.Errorf("unrecognized prefix")
	}
}

type splitState uint

const (
	// Begin is the beginning state of a field split
	Begin splitState = iota
	// InBracket is the state of a field split inside a bracket
	InBracket
	// InQuote is the state of a field split inside a quote
	InQuote
	// OutQuote is the state of a field split outside a quote
	OutQuote
	// OutBracket is the state of a field split outside a bracket
	OutBracket
	// InUnbracketedToken is the state field split on any token outside brackets
	InUnbracketedToken
)

func fromJSONDot(s string) ([]string, error) {
	fields := make([]string, 0, 1)

	state := Begin
	var quoteChar rune
	var tokenStart int

	for i, c := range s {
		switch state {
		case Begin:
			if c == '[' {
				state = InBracket
				continue
			}
			tokenStart = i
			state = InUnbracketedToken
		case InBracket:
			if !(c == '\'' || c == '"') {
				return nil, fmt.Errorf("strings in brackets must be surrounded by quotes")
			}
			state = InQuote
			quoteChar = c
			tokenStart = i + 1
		case InQuote:
			if c == quoteChar {
				fields = append(fields, s[tokenStart:i])
				state = OutQuote
			}
		case OutQuote:
			if c != ']' {
				return nil, fmt.Errorf("found characters between closed quote and closing bracket")
			}
			state = OutBracket
		case OutBracket:
			switch c {
			case '.':
				state = InUnbracketedToken
				tokenStart = i + 1
			case '[':
				state = InBracket
			default:
				return nil, fmt.Errorf("bracketed access must be followed by a dot or another bracketed access")
			}
		case InUnbracketedToken:
			if c == '.' {
				fields = append(fields, s[tokenStart:i])
				tokenStart = i + 1
			} else if c == '[' {
				fields = append(fields, s[tokenStart:i])
				state = InBracket
			}
		}
	}

	switch state {
	case InBracket, OutQuote:
		return nil, fmt.Errorf("found unclosed left bracket")
	case InQuote:
		if quoteChar == '"' {
			return nil, fmt.Errorf("found unclosed double quote")
		}
		return nil, fmt.Errorf("found unclosed single quote")
	case InUnbracketedToken:
		fields = append(fields, s[tokenStart:])
	case Begin, OutBracket:
		// shouldn't be possible
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("fields size is 0")
	}

	return fields, nil
}

// toJSONDot returns the JSON dot notation for a field.
func toJSONDot(prefix string, keys []string) string {
	if len(keys) == 0 {
		return prefix
	}

	containsDots := false
	for _, key := range keys {
		if strings.Contains(key, ".") {
			containsDots = true
		}
	}

	var b strings.Builder
	b.WriteString(prefix)
	if containsDots {
		for _, key := range keys {
			b.WriteString(`['`)
			b.WriteString(key)
			b.WriteString(`']`)
		}
	} else {
		b.WriteString(".")
		for i, key := range keys {
			if i != 0 {
				b.WriteString(".")
			}
			b.WriteString(key)
		}
	}

	return b.String()
}

// getNestedMap will get a nested map assigned to a key.
// If the map does not exist, it will create and return it.
func getNestedMap(currentMap map[string]any, key string) map[string]any {
	currentValue, ok := currentMap[key]
	if !ok {
		currentMap[key] = map[string]any{}
	}

	nextMap, ok := currentValue.(map[string]any)
	if !ok {
		nextMap = map[string]any{}
		currentMap[key] = nextMap
	}

	return nextMap
}
