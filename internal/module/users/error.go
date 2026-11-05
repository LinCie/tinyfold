package users

import (
	"errors"

	"tinyfold/internal/pkg/validation"
)

var (
	ErrInvalidUser = errors.New("invalid user")
	ErrNotFound    = errors.New("user not found")
)

// ValidationError describes an invalid user field. It matches ErrInvalidUser.
type ValidationError = validation.Error
