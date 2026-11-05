package posts

import (
	"errors"

	"tinyfold/internal/pkg/validation"
)

var (
	ErrInvalidPost = errors.New("invalid post")
	ErrNotFound    = errors.New("post not found")
)

// ValidationError describes an invalid post field. It matches ErrInvalidPost.
type ValidationError = validation.Error
