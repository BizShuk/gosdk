package string

import (
	"errors"
	"fmt"
)

// errMinLenFailed is the sentinel returned by MinLen when the wrapped
// string is shorter than the configured minimum length.
var errMinLenFailed = errors.New("validator/string: value is shorter than minimum length")

// MinLenError is the typed error returned by MinLen. It carries the
// wrapped value, the configured minimum, and the actual length so callers
// can inspect all three via errors.As.
type MinLenError struct {
	Value string
	Min   int
	Got   int
}

// Error formats the error using the sentinel prefix plus the actual and
// required lengths.
func (e *MinLenError) Error() string {
	return fmt.Sprintf("%s (got %d, want >= %d)", errMinLenFailed, e.Got, e.Min)
}

// Is bridges the typed error to the sentinel so errors.Is works
// regardless of how the error was produced.
func (e *MinLenError) Is(target error) bool {
	return target == errMinLenFailed
}

// MinLen fails when len(value) < min. MinLen counts bytes, not runes —
// if the input can be multi-byte UTF-8, use a rune-aware validator
// alongside this one.
type MinLen struct {
	value string
	min   int
}

// NewMinLen wraps a string with a minimum length constraint. A
// non-positive min means every non-empty string passes.
func NewMinLen(value string, min int) *MinLen {
	return &MinLen{value: value, min: min}
}

// Value returns the wrapped string.
func (m *MinLen) Value() string { return m.value }

// Min returns the minimum length requirement.
func (m *MinLen) Min() int { return m.min }

// Validate returns a *MinLenError when len(value) < min.
func (m *MinLen) Validate() error {
	if got := len(m.value); got < m.min {
		return &MinLenError{Value: m.value, Min: m.min, Got: got}
	}
	return nil
}
