// Package validation contains shared module validation errors.
package validation

import "fmt"

// Error describes an invalid field and matches the supplied sentinel error.
type Error struct {
	Field  string
	Reason string
	cause  error
}

// New creates a validation error that unwraps to invalid.
func New(invalid error, field, reason string) *Error {
	return &Error{Field: field, Reason: reason, cause: invalid}
}

func (e *Error) Error() string {
	if e == nil {
		return "invalid input"
	}
	if e.cause == nil {
		return fmt.Sprintf("invalid input: %s %s", e.Field, e.Reason)
	}
	return fmt.Sprintf("%s: %s %s", e.cause, e.Field, e.Reason)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
