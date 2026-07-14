package string

import (
	"errors"
	"fmt"
)

// errEqualToFailed is the sentinel returned by EqualTo when the wrapped
// string does not match the expected value.
var errEqualToFailed = errors.New("validator/string: value does not equal expected")

// EqualToError is the typed error returned by EqualTo. It carries the
// wrapped value and the expected value so callers can compare them
// programmatically (e.g. password vs. password-confirm fields).
type EqualToError struct {
	Value    string
	Expected string
}

// Error formats the error using the sentinel prefix plus the actual
// and expected values.
func (e *EqualToError) Error() string {
	return fmt.Sprintf("%s (got %q, want %q)", errEqualToFailed, e.Value, e.Expected)
}

// Is bridges the typed error to the sentinel.
func (e *EqualToError) Is(target error) bool {
	return target == errEqualToFailed
}

// EqualTo fails when value != expected. Comparison is byte-exact and
// case-sensitive. A typical use is comparing a password field with a
// password-confirm field in form validation.
type EqualTo struct {
	value    string
	expected string
}

// NewEqualTo wraps a string and the value it must equal.
func NewEqualTo(value, expected string) *EqualTo {
	return &EqualTo{value: value, expected: expected}
}

// Value returns the wrapped string.
func (e *EqualTo) Value() string { return e.value }

// Expected returns the configured expected value.
func (e *EqualTo) Expected() string { return e.expected }

// Validate returns an *EqualToError when value != expected.
func (e *EqualTo) Validate() error {
	if e.value != e.expected {
		return &EqualToError{Value: e.value, Expected: e.expected}
	}
	return nil
}
