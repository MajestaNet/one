package dataengine

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a record or object row is missing.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned for duplicate external IDs or unique collisions.
var ErrConflict = errors.New("conflict")

// ValidationError is a client-facing field/query validation failure.
type ValidationError struct {
	Message string
	Details map[string]any
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "validation error"
	}
	return e.Message
}

func validationErrorf(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}
