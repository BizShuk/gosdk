package string

import (
	"errors"
	"fmt"
)

// errNotEqualToFailed is the sentinel returned by NotEqualTo when the
// wrapped string equals the forbidden value.
var errNotEqualToFailed = errors.New("validator/string: value must not equal forbidden")

// NotEqualToError is the typed error returned by NotEqualTo. It carries
// the wrapped value and the forbidden value.
type NotEqualToError struct {
	Value     string
	Forbidden string
}

// Error formats the error using the sentinel prefix plus the actual
// and forbidden values.
func (e *NotEqualToError) Error() string {
	return fmt.Sprintf("%s (got %q, must not equal %q)", errNotEqualToFailed, e.Value, e.Forbidden)
}

// Is bridges the typed error to the sentinel.
func (e *NotEqualToError) Is(target error) bool {
	return target == errNotEqualToFailed
}

// NotEqualTo fails when value == forbidden. A typical use is rejecting
// a value equal to a placeholder, sentinel, or some other known-bad
// string in form validation.
type NotEqualTo struct {
	value     string
	forbidden string
}

// NewNotEqualTo wraps a string and the value it must not equal.
func NewNotEqualTo(value, forbidden string) *NotEqualTo {
	return &NotEqualTo{value: value, forbidden: forbidden}
}

// Value returns the wrapped string.
func (n *NotEqualTo) Value() string { return n.value }

// Forbidden returns the configured forbidden value.
func (n *NotEqualTo) Forbidden() string { return n.forbidden }

// Validate returns a *NotEqualToError when value == forbidden.
func (n *NotEqualTo) Validate() error {
	if n.value == n.forbidden {
		return &NotEqualToError{Value: n.value, Forbidden: n.forbidden}
	}
	return nil
}
