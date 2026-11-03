package users

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidUser = errors.New("invalid user")
	ErrNotFound    = errors.New("user not found")
)

// ValidationError describes an invalid user field. It matches ErrInvalidUser.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidUser.Error()
	}
	return fmt.Sprintf("%s: %s %s", ErrInvalidUser, e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidUser
}
