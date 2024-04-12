package entity

import (
	"database/sql/driver"

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

func (id Id) String() string {
	return id.val.String()
}

func (id Id) Value() (driver.Value, error) {
	return id.String(), nil
}
