package entity

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

// Represents a limit configuration as a map of period
// and limit value. This is again a value object and should
// be treated as such.
// Implements (driver.Valuer and sql.Scanner) interfaces for
// databases and (json.Marshaler and json.Unmarshaler)
// interfaces for JSON.
type LimitConfig struct {
	cfg map[Period]LimitValue
}

func (lc LimitConfig) Map() map[Period]LimitValue {
	return lc.cfg
}

func (lc LimitConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]LimitValue)

	for period, limitValue := range lc.cfg {
		m[period.String()] = limitValue
	}

	return json.Marshal(m)
}

func (lc *LimitConfig) UnmarshalJSON(data []byte) error {
	// Directly unmarshalling to map[Period]LimitValue is not working
	// inspite of writing JSON serializers and deserializers for Period.
	// Perhaps because it is a key?
	m1 := make(map[string]LimitValue)
	if err := json.Unmarshal(data, &m1); err != nil {
		return err
	}

	m2 := make(map[Period]LimitValue)
	for str, limitValue := range m1 {
		period, err := NewPeriod(str)
		if err != nil {
			return err
		}
		m2[period] = limitValue
	}

	(*lc).cfg = m2
	return nil
}

func (lc LimitConfig) Value() (driver.Value, error) {
	bytes, err := lc.MarshalJSON()
	if err != nil {
		return nil, err
	}

	// Return a string to be stored in the database
	return string(bytes), nil
}

func (lc *LimitConfig) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	return lc.UnmarshalJSON(bytes)
}
