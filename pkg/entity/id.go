package entity

import (
	"database/sql/driver"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/google/uuid"
)

type Id struct {
	val uuid.UUID
}

func NewId() Id {
	val, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	return Id{
		val: val,
	}
}

func NewIdFromString(value string) (Id, error) {
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

func (id Id) Value() (driver.Value, error) {
	return id.String(), nil
}

func (id *Id) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	valString, ok := value.(string)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	newId, err := NewIdFromString(valString)
	if err != nil {
		return err
	}

	*id = newId
	return nil
}
