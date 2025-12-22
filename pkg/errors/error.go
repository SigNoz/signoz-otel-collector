package errors

import "errors"

type Error struct {
	error
	isRetryable bool
}

var _ error = (*Error)(nil)

func New(origErr error) error {
	return &Error{error: origErr, isRetryable: false}
}

func NewRetryableError(origErr error) error {
	return &Error{error: origErr, isRetryable: true}
}

func (e *Error) Unwrap() error {
	if e.error != nil {
		return e.error
	}

	return nil
}

func (e *Error) IsRetryable() bool {
	return e.isRetryable
}

func As(err error, target any) bool {
	return errors.As(err, target)
}

func Is(err error, target error) bool {
	return errors.Is(err, target)
}
