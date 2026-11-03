package posts

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPost = errors.New("invalid post")
	ErrNotFound    = errors.New("post not found")
)

// ValidationError describes an invalid post field. It matches ErrInvalidPost.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidPost.Error()
	}
	return fmt.Sprintf("%s: %s %s", ErrInvalidPost, e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidPost
}
