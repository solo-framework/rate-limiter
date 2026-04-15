package tests

import (
	"errors"
	"net/http"
	"ratelimiter/control"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError(t *testing.T) {
	inner := errors.New("inner")
	err := control.NewValidationError("bad input", inner)

	if err.Error() != "bad input" {
		t.Fatalf("expected message 'bad input', got %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped error to be returned by errors.Is")
	}
	if err.GetStatus() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.GetStatus())
	}

	assert.True(t, err.NeedLogging(), "expected NeedToBeLogged true")

}

func TestInternalError(t *testing.T) {
	inner := errors.New("inner")
	err := control.NewInternalError("internal", inner)

	if err.Error() != "internal" {
		t.Fatalf("expected message 'internal', got %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped error to be returned by errors.Is")
	}
	if err.GetStatus() != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, err.GetStatus())
	}
	if !err.NeedLogging() {
		t.Fatalf("expected NeedToBeLogged true")
	}
}

func TestNotFoundError(t *testing.T) {
	inner := errors.New("inner")
	err := control.NewNotFoundError("not found", inner)

	if err.Error() != "not found" {
		t.Fatalf("expected message 'not found', got %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped error to be returned by errors.Is")
	}
	if err.GetStatus() != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, err.GetStatus())
	}
	if err.NeedLogging() {
		t.Fatalf("expected NeedToBeLogged false")
	}
}

func TestLogicError(t *testing.T) {
	inner := errors.New("inner")
	err := control.NewLogicError("logic", inner, http.StatusTeapot)

	if err.Error() != "logic" {
		t.Fatalf("expected message 'logic', got %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped error to be returned by errors.Is")
	}
	if err.GetStatus() != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, err.GetStatus())
	}
	if !err.NeedLogging() {
		t.Fatalf("expected NeedToBeLogged true")
	}
}

var (
	_ control.IKnownError = (*control.ValidationError)(nil)
	_ control.IKnownError = (*control.InternalError)(nil)
	_ control.IKnownError = (*control.NotFoundError)(nil)
	_ control.IKnownError = (*control.LogicError)(nil)
)
