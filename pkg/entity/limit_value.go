package entity

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

// LimitValue represents a limit value with a count and size.
// This would typically correspond to signal counts and sizes.
// Please note that attaching json tags or db tags directly to
// value objects is a bad practice as it polutes the entity layer
// with infrastructure/adapter knowledge. However, in the case of
// limit value, the assumption is that these json tags make common
// sense and enforce a strict requirement on JSON serialization and
// deserialization.
type LimitValue struct {
	Count int `json:"count"`
	Size  int `json:"size"`
}

func NewLimitValue(count int, size int) LimitValue {
	return LimitValue{
		Count: count,
		Size:  size,
	}
}

func (lv LimitValue) Value() (driver.Value, error) {
	bytes, err := json.Marshal(lv)
	if err != nil {
		return nil, err
	}

	return string(bytes), nil
}

func (lv *LimitValue) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	valString, ok := value.(string)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	return json.Unmarshal([]byte(valString), lv)
}

// Returns a new LimitValue by subtracting both count and size.
// Always return a new value object instead of changing state of
// an existing value object.
func (lv LimitValue) Subtract(input LimitValue) LimitValue {
	return NewLimitValue(lv.Count-input.Count, lv.Size-input.Size)
}

// Determines whether the limit value is less than num by checking
// if the count OR the size is less than num.
func (lv LimitValue) LessThan(num int) bool {
	if lv.Count < num || lv.Size < num {
		return true
	}

	return false
}
