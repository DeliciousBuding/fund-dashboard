package portfolio

import (
	"errors"
	"fmt"
)

// ValidationError marks a client-input failure. Its message is short, stable
// and safe to return to the caller; it never carries SQL/driver noise, so HTTP
// handlers can map it to 400 while internal/DB failures map to 500.
type ValidationError struct {
	message string
}

// Error implements error and returns only the user-facing message.
func (e *ValidationError) Error() string { return e.message }

// NewValidationError builds a ValidationError with a formatted user-facing
// message. Unlike fmt.Errorf, %w is not supported — callers must render any
// underlying cause with %v so the client-visible message stays safe.
func NewValidationError(format string, args ...any) error {
	return &ValidationError{message: fmt.Sprintf(format, args...)}
}

// IsValidationError reports whether err is or wraps a ValidationError.
func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}
