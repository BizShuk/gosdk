package string

import (
	"errors"
	"fmt"
)

// errOneOfFailed is the sentinel returned by OneOf when the wrapped
// string is not in the allowed set.
var errOneOfFailed = errors.New("validator/string: value is not in the allowed set")

// OneOfError is the typed error returned by OneOf. It carries the
// wrapped value and a copy of the allowed set.
type OneOfError struct {
	Value   string
	Allowed []string
}

// Error formats the error using the sentinel prefix plus the input
// value and the allowed set.
func (e *OneOfError) Error() string {
	return fmt.Sprintf("%s (got %q, want one of %v)", errOneOfFailed, e.Value, e.Allowed)
}

// Is bridges the typed error to the sentinel.
func (e *OneOfError) Is(target error) bool {
	return target == errOneOfFailed
}

// OneOf fails when the wrapped string is not present in the allowed
// set. Comparison is case-sensitive — use a lowercase canonicalisation
// step if case-insensitive matching is required.
//
// The allowed slice is defensively copied at construction so caller
// mutations after NewOneOf cannot affect validation.
type OneOf struct {
	value   string
	allowed []string
}

// NewOneOf wraps a string and the set of allowed values. Passing an
// empty allowed set means "always fails" since no value is contained
// in an empty set.
func NewOneOf(value string, allowed ...string) *OneOf {
	cp := make([]string, len(allowed))
	copy(cp, allowed)
	return &OneOf{value: value, allowed: cp}
}

// Value returns the wrapped string.
func (o *OneOf) Value() string { return o.value }

// Allowed returns a copy of the allowed set. The returned slice is safe
// to mutate without affecting the validator.
func (o *OneOf) Allowed() []string {
	cp := make([]string, len(o.allowed))
	copy(cp, o.allowed)
	return cp
}

// Validate returns a *OneOfError when value is not in the allowed set.
// The Allowed field of the error holds a fresh defensive copy of the
// allowed set, so the caller can pass it around safely.
func (o *OneOf) Validate() error {
	for _, a := range o.allowed {
		if a == o.value {
			return nil
		}
	}
	return &OneOfError{Value: o.value, Allowed: o.Allowed()}
}
