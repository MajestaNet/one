package deploy

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a deploy resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned for trust/authz failures.
var ErrForbidden = errors.New("forbidden")

// ErrBusy is returned when deploy-class work is at JOB_SLOTS_DEPLOY / DEPLOY_QUEUE_MAX.
var ErrBusy = errors.New("deploy busy")

// BusyError is a 429 DEPLOY_BUSY failure.
type BusyError struct {
	Message string
	Slots   int
	Queue   int
}

func (e *BusyError) Error() string {
	if e == nil || e.Message == "" {
		return ErrBusy.Error()
	}
	return e.Message
}

func (e *BusyError) Unwrap() error { return ErrBusy }

func newBusyError(slots, queue int) *BusyError {
	return &BusyError{
		Slots: slots,
		Queue: queue,
		Message: fmt.Sprintf(
			"org validate/deploy/test is at JOB_SLOTS_DEPLOY=%d and queue depth limit DEPLOY_QUEUE_MAX=%d",
			slots, queue,
		),
	}
}

// ValidationError wraps a human-readable validation failure.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newValidationError(msg string) *ValidationError {
	return &ValidationError{Message: msg}
}

func newValidationErrorf(format string, args ...any) *ValidationError {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

func newForbiddenError(msg string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, msg)
}

func newNotFoundError(msg string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, msg)
}
