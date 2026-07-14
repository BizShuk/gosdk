package numeric

import (
	"errors"
	"fmt"
)

// errRangeFailed is the sentinel returned by Range when the wrapped
// value lies outside the closed interval [min, max].
var errRangeFailed = errors.New("validator/numeric: value is outside the allowed range")

// RangeError is the typed error returned by Range. It carries the
// wrapped value and both bounds so the caller can see which side of
// the interval was breached.
type RangeError struct {
	Value int
	Min   int
	Max   int
}

// Error formats the error using the sentinel prefix plus the actual
// value and both bounds.
func (e *RangeError) Error() string {
	return fmt.Sprintf("%s (got %d, want in [%d, %d])", errRangeFailed, e.Value, e.Min, e.Max)
}

// Is bridges the typed error to the sentinel.
func (e *RangeError) Is(target error) bool {
	return target == errRangeFailed
}

// Range fails when value < min OR value > max. Boundaries are inclusive
// (both min and max pass). Behaviour is undefined if min > max — the
// caller is responsible for keeping the bounds sane; an inverted range
// causes every value to fail, which is correct in practice but noisy.
type Range struct {
	value int
	min   int
	max   int
}

// NewRange wraps an int with a closed-interval range constraint.
func NewRange(value, min, max int) *Range {
	return &Range{value: value, min: min, max: max}
}

// Value returns the wrapped int.
func (r *Range) Value() int { return r.value }

// Min returns the lower bound.
func (r *Range) Min() int { return r.min }

// Max returns the upper bound.
func (r *Range) Max() int { return r.max }

// Validate returns a *RangeError when value is outside [min, max].
func (r *Range) Validate() error {
	if r.value < r.min || r.value > r.max {
		return &RangeError{Value: r.value, Min: r.min, Max: r.max}
	}
	return nil
}
