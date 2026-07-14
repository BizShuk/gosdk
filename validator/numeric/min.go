package numeric

import (
	"errors"
	"fmt"
)

// errMinFailed is the sentinel returned by Min when the wrapped value
// is smaller than the configured minimum.
var errMinFailed = errors.New("validator/numeric: value is smaller than minimum")

// MinError is the typed error returned by Min. It carries the wrapped
// value and the configured minimum.
type MinError struct {
	Value int
	Min   int
}

// Error formats the error using the sentinel prefix plus the actual
// value and the required minimum.
func (e *MinError) Error() string {
	return fmt.Sprintf("%s (got %d, want >= %d)", errMinFailed, e.Value, e.Min)
}

// Is bridges the typed error to the sentinel.
func (e *MinError) Is(target error) bool {
	return target == errMinFailed
}

// Min fails when value < min. Boundary (value == min) passes.
type Min struct {
	value int
	min   int
}

// NewMin wraps an int with a minimum-bound constraint.
func NewMin(value, min int) *Min {
	return &Min{value: value, min: min}
}

// Value returns the wrapped int.
func (m *Min) Value() int { return m.value }

// Min returns the configured minimum.
func (m *Min) Min() int { return m.min }

// Validate returns a *MinError when value < min.
func (m *Min) Validate() error {
	if m.value < m.min {
		return &MinError{Value: m.value, Min: m.min}
	}
	return nil
}
