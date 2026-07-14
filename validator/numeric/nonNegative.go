package numeric

import (
	"errors"
	"fmt"
)

// errNotNonNegative is the sentinel returned by NonNegative when the
// wrapped value is negative.
var errNotNonNegative = errors.New("validator/numeric: value must be non-negative")

// NonNegativeError is the typed error returned by NonNegative. It
// carries the wrapped value.
type NonNegativeError struct {
	Value int
}

// Error formats the error using the sentinel prefix plus the actual value.
func (e *NonNegativeError) Error() string {
	return fmt.Sprintf("%s (got %d)", errNotNonNegative, e.Value)
}

// Is bridges the typed error to the sentinel.
func (e *NonNegativeError) Is(target error) bool {
	return target == errNotNonNegative
}

// NonNegative fails when value < 0. Zero passes.
type NonNegative struct {
	value int
}

// NewNonNegative wraps an int in a NonNegative validator.
func NewNonNegative(value int) *NonNegative {
	return &NonNegative{value: value}
}

// Value returns the wrapped int.
func (n *NonNegative) Value() int { return n.value }

// Validate returns a *NonNegativeError when value < 0.
func (n *NonNegative) Validate() error {
	if n.value < 0 {
		return &NonNegativeError{Value: n.value}
	}
	return nil
}
