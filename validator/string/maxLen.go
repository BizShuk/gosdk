package string

import (
	"errors"
	"fmt"
)

// errMaxLenFailed is the sentinel returned by MaxLen when the wrapped
// string is longer than the configured maximum length.
var errMaxLenFailed = errors.New("validator/string: value is longer than maximum length")

// MaxLenError is the typed error returned by MaxLen. It carries the
// wrapped value, the configured maximum, and the actual length.
type MaxLenError struct {
	Value string
	Max   int
	Got   int
}

// Error formats the error using the sentinel prefix plus the actual and
// allowed maximum lengths.
func (e *MaxLenError) Error() string {
	return fmt.Sprintf("%s (got %d, want <= %d)", errMaxLenFailed, e.Got, e.Max)
}

// Is bridges the typed error to the sentinel.
func (e *MaxLenError) Is(target error) bool {
	return target == errMaxLenFailed
}

// MaxLen fails when len(value) > max. Pair with MinLen via
// validator.Validator to express a length range.
type MaxLen struct {
	value string
	max   int
}

// NewMaxLen wraps a string with a maximum length constraint. A negative
// max means every string fails.
func NewMaxLen(value string, max int) *MaxLen {
	return &MaxLen{value: value, max: max}
}

// Value returns the wrapped string.
func (m *MaxLen) Value() string { return m.value }

// Max returns the maximum length requirement.
func (m *MaxLen) Max() int { return m.max }

// Validate returns a *MaxLenError when len(value) > max.
func (m *MaxLen) Validate() error {
	if got := len(m.value); got > m.max {
		return &MaxLenError{Value: m.value, Max: m.max, Got: got}
	}
	return nil
}
