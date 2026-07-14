package numeric

import (
	"errors"
	"fmt"
)

// errMaxFailed is the sentinel returned by Max when the wrapped value
// is greater than the configured maximum.
var errMaxFailed = errors.New("validator/numeric: value is greater than maximum")

// MaxError is the typed error returned by Max. It carries the wrapped
// value and the configured maximum.
type MaxError struct {
	Value int
	Max   int
}

// Error formats the error using the sentinel prefix plus the actual
// value and the allowed maximum.
func (e *MaxError) Error() string {
	return fmt.Sprintf("%s (got %d, want <= %d)", errMaxFailed, e.Value, e.Max)
}

// Is bridges the typed error to the sentinel.
func (e *MaxError) Is(target error) bool {
	return target == errMaxFailed
}

// Max fails when value > max. Boundary (value == max) passes.
type Max struct {
	value int
	max   int
}

// NewMax wraps an int with a maximum-bound constraint.
func NewMax(value, max int) *Max {
	return &Max{value: value, max: max}
}

// Value returns the wrapped int.
func (m *Max) Value() int { return m.value }

// Max returns the configured maximum.
func (m *Max) Max() int { return m.max }

// Validate returns a *MaxError when value > max.
func (m *Max) Validate() error {
	if m.value > m.max {
		return &MaxError{Value: m.value, Max: m.max}
	}
	return nil
}
