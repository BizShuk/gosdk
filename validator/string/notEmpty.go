package string

import (
	"errors"
	"fmt"
)

// errNotEmptyFailed is the sentinel returned by NotEmpty when the wrapped
// string is empty. Callers can use errors.Is to detect this condition.
var errNotEmptyFailed = errors.New("validator/string: value must not be empty")

// NotEmptyError is the typed error returned by NotEmpty when validation
// fails. It carries the wrapped value so callers can inspect it
// programmatically via errors.As.
type NotEmptyError struct {
	Value string
}

// Error formats the error using the sentinel prefix and the actual value,
// so log output reads naturally while errors.Is(err, errNotEmptyFailed)
// and errors.As(err, &NotEmptyError{}) both succeed.
func (e *NotEmptyError) Error() string {
	return fmt.Sprintf("%s (got %q)", errNotEmptyFailed, e.Value)
}

// Is reports whether target is the sentinel, so errors.Is works
// regardless of whether the error came back as the sentinel itself or
// wrapped in *NotEmptyError.
func (e *NotEmptyError) Is(target error) bool {
	return target == errNotEmptyFailed
}

// NotEmpty fails when the wrapped string is the empty string "". A
// whitespace-only string still passes — pair it with MinLen and a Pattern
// validator when whitespace trimming is also required.
type NotEmpty struct {
	value string
}

// NewNotEmpty wraps a string in a NotEmpty validator.
func NewNotEmpty(value string) *NotEmpty {
	return &NotEmpty{value: value}
}

// Value returns the wrapped string.
func (n *NotEmpty) Value() string { return n.value }

// Validate returns a *NotEmptyError when the wrapped string has zero
// length, and nil otherwise.
func (n *NotEmpty) Validate() error {
	if n.value == "" {
		return &NotEmptyError{Value: n.value}
	}
	return nil
}
