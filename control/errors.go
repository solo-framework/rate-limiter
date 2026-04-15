package control

import (
	"errors"
	"net/http"
)

var ErrJsonDecode = errors.New("json decode error")
var ErrWriteResponse = errors.New("write response error")

type IKnownError interface {

	// GetStatus returns HTTP status for client error response.
	GetStatus() int

	// Unwrap gives access to the internal Err value.
	Unwrap() error

	// NeedLogging reports whether this error should be logged.
	NeedLogging() bool

	Error() string
}

// baseError is the base type for all custom errors.
type baseError struct {
	Err     error
	Message string // this message is shown to the user
}

func (s *baseError) Error() string {
	return s.Message
}

func (s *baseError) Unwrap() error {
	return s.Err
}

func (s *baseError) GetStatus() int {
	panic("baseError: define GetStatus method!")
}

// ValidationError is for input validation failures.
type ValidationError struct {
	baseError
}

func NewValidationError(message string, err error) *ValidationError {
	return &ValidationError{
		baseError: baseError{Err: err, Message: message},
	}
}

func (s *ValidationError) GetStatus() int {
	return http.StatusBadRequest
}

func (s *ValidationError) NeedLogging() bool {
	return true
}

// NewInternalError is for unexpected internal errors.
func NewInternalError(message string, err error) *InternalError {
	return &InternalError{
		baseError: baseError{Err: err, Message: message},
	}
}

type InternalError struct {
	baseError
}

func (s *InternalError) NeedLogging() bool {
	return true
}

func (s *InternalError) GetStatus() int {
	return http.StatusInternalServerError
}

// NewNotFoundError is used when something is not found.
func NewNotFoundError(message string, err error) *NotFoundError {
	return &NotFoundError{
		baseError: baseError{Err: err, Message: message},
	}
}

type NotFoundError struct {
	baseError
}

func (s *NotFoundError) NeedLogging() bool {
	return false
}

func (s *NotFoundError) GetStatus() int {
	return http.StatusNotFound
}

// LogicError represents business-logic errors.
type LogicError struct {
	baseError
	Status int
}

func NewLogicError(message string, err error, status int) *LogicError {
	return &LogicError{
		baseError: baseError{Err: err, Message: message},
		Status:    status,
	}
}

func (s *LogicError) NeedLogging() bool {
	return true
}

func (s *LogicError) GetStatus() int {
	return s.Status
}
