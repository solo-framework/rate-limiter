package errors

import "errors"

var ErrInvalidData = errors.New("invalid data")

// helper for wrapping errors
func NewError(message string, err error) *Error {
	return &Error{
		Err:     err,
		Message: message,
	}
}

type Error struct {
	Err     error
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}
