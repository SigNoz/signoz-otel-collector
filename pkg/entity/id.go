package entity

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/google/uuid"
)

// Id represents a unique identifier for domain entities. It is a value object
// and should be treated as such.
// It implements the driver.Valuer and sql.Scanner interfaces for
// working with databases,and the json.Marshaler and json.Unmarshaler
// interfaces for JSON serialization and deserialization.
type Id struct {
	val uuid.UUID
}

func GenerateId() Id {
	val, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	return Id{
		val: val,
	}
}

func NewId(value string) (Id, error) {
	val, err := uuid.Parse(value)
	if err != nil {
		return Id{}, err
	}

	return Id{
		val: val,
	}, nil
}

func (id Id) String() string {
	return id.val.String()
}

func (id Id) IsZero() bool {
	return id == Id{}
}

func (id Id) Value() (driver.Value, error) {
	return id.String(), nil
}

func (id *Id) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	str, ok := value.(string)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	newId, err := NewId(str)
	if err != nil {
		return err
	}

	*id = newId
	return nil
}

func (id Id) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

func (id *Id) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	i, err := NewId(str)
	if err != nil {
		return err
	}

	*id = i
	return nil
}
