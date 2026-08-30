package actions

import (
	"fmt"
	"net/http"
)

// Error is a Client-facing platform action failure (ADR-029 error codes).
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *Error) Error() string {
	if e == nil {
		return "action error"
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func errNotFound(apiName string) *Error {
	return &Error{
		Status:  http.StatusNotFound,
		Code:    "ACTION_NOT_FOUND",
		Message: fmt.Sprintf("unknown action %q", apiName),
	}
}

func errPackageNotEnabled(packageName, option string) *Error {
	msg := fmt.Sprintf("required package %s is not enabled", packageName)
	details := map[string]any{"packageName": packageName}
	if option != "" {
		msg = fmt.Sprintf("package %s is not enabled for option %s", packageName, option)
		details["option"] = option
	}
	return &Error{
		Status:  http.StatusConflict,
		Code:    "PACKAGE_NOT_ENABLED",
		Message: msg,
		Details: details,
	}
}

func errForbidden(message string) *Error {
	if message == "" {
		message = "forbidden"
	}
	return &Error{
		Status:  http.StatusForbidden,
		Code:    "FORBIDDEN",
		Message: message,
	}
}

func errValidation(message string) *Error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "VALIDATION_FAILED",
		Message: message,
	}
}

func errAsyncOnly() *Error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "VALIDATION_FAILED",
		Message: "invokeAction is not allowed in sync automations for async-only actions",
	}
}
