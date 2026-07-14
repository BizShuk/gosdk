// Package numeric provides int-typed validators that satisfy the
// gosdk/validator IValidator contract.
//
// Every type in this package wraps an int captured at construction time
// and exposes Validate() error. They are designed to be combined with
// validator.Validator to express compound constraints over a single
// integer value. Each validator returns a typed *Error that satisfies
// both errors.Is (against the package sentinel) and errors.As (for
// programmatic inspection of the failure context).
package numeric

import (
	"errors"
	"fmt"
)

// errNotPositive is the sentinel returned by Positive when the wrapped
// value is zero or negative.
var errNotPositive = errors.New("validator/numeric: value must be positive")

// PositiveError is the typed error returned by Positive. It carries the
// wrapped value so callers can inspect it via errors.As.
type PositiveError struct {
	Value int
}

// Error formats the error using the sentinel prefix plus the actual value.
func (e *PositiveError) Error() string {
	return fmt.Sprintf("%s (got %d)", errNotPositive, e.Value)
}

// Is bridges the typed error to the sentinel.
func (e *PositiveError) Is(target error) bool {
	return target == errNotPositive
}

// Positive fails when value <= 0.
type Positive struct {
	value int
}

// NewPositive wraps an int in a Positive validator.
func NewPositive(value int) *Positive {
	return &Positive{value: value}
}

// Value returns the wrapped int.
func (p *Positive) Value() int { return p.value }

// Validate returns a *PositiveError when value <= 0.
func (p *Positive) Validate() error {
	if p.value <= 0 {
		return &PositiveError{Value: p.value}
	}
	return nil
}
