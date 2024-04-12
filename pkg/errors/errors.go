package errors

import (
	"fmt"
)

type Type struct {
	name string
}

var (
	TypeInvalidInput = Type{name: "invalid-input"}
	TypeInternal     = Type{name: "internal"}
	TypeUnsupported  = Type{name: "unsupported"}
	TypeNotFound     = Type{name: "not-found"}
)

func (t Type) String() string {
	return t.name
}

type base struct {
	// t denotes the type of the error
	t Type
	// i contains error message passed through errors.New
	i string
	// e is the actual error being wrapped
	e error
}

func (b *base) Error() string {
	if b.e != nil {
		return b.e.Error()
	}

	return fmt.Sprintf("%s: %s", b.t.name, b.i)
}

func New(t Type, info string) error {
	return &base{
		t: t,
		i: info,
		e: nil,
	}
}

func Newf(t Type, format string, args ...interface{}) error {
	return &base{
		t: t,
		i: fmt.Sprintf(format, args...),
		e: nil,
	}
}

func Wrapf(cause error, t Type, format string, args ...interface{}) error {
	return &base{
		t: t,
		i: fmt.Sprintf(format, args...),
		e: cause,
	}
}

// Unwrap error into base
func Unwrapb(cause error) (Type, string, error) {
	base, ok := cause.(*base)
	if ok {
		return base.t, base.i, base.e
	}

	return TypeInternal, cause.Error(), cause
}
