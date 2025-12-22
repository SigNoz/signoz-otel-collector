package migrate

import "errors"

type base struct {
	error
	isRetryable bool
}

var _ error = (*base)(nil)

func (e *base) Unwrap() error {
	if e.error != nil {
		return e.error
	}

	return nil
}

func (e *base) IsRetryable() bool {
	return e.isRetryable
}

func New(origErr error) *base {
	return &base{error: origErr, isRetryable: false}
}

func NewRetryableError(origErr error) *base {
	return &base{error: origErr, isRetryable: true}
}

func Unwrapb(err error) *base {
	if b, ok := err.(*base); ok {
		return b
	}

	return New(err)
}

func As(err error, target any) bool {
	return errors.As(err, target)
}

func Is(err error, target error) bool {
	return errors.Is(err, target)
}
