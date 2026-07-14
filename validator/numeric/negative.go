package numeric

import (
	"errors"
	"fmt"
)

// errNotNegative is the sentinel returned by Negative when the wrapped
// value is zero or positive.
var errNotNegative = errors.New("validator/numeric: value must be negative")

// NegativeError is the typed error returned by Negative. It carries the
// wrapped value.
type NegativeError struct {
	Value int
}

// Error formats the error using the sentinel prefix plus the actual value.
func (e *NegativeError) Error() string {
	return fmt.Sprintf("%s (got %d)", errNotNegative, e.Value)
}

// Is bridges the typed error to the sentinel.
func (e *NegativeError) Is(target error) bool {
	return target == errNotNegative
}

// Negative fails when value >= 0.
type Negative struct {
	value int
}

// NewNegative wraps an int in a Negative validator.
func NewNegative(value int) *Negative {
	return &Negative{value: value}
}

// Value returns the wrapped int.
func (n *Negative) Value() int { return n.value }

// Validate returns a *NegativeError when value >= 0.
func (n *Negative) Validate() error {
	if n.value >= 0 {
		return &NegativeError{Value: n.value}
	}
	return nil
}
