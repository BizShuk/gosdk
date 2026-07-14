package string

import (
	"errors"
	"fmt"
	"regexp"
)

// errPatternMismatch is the sentinel returned by Pattern when the wrapped
// string does not match the configured regular expression.
var errPatternMismatch = errors.New("validator/string: value does not match pattern")

// PatternError is the typed error returned by Pattern. It carries the
// wrapped value and the regex source for context.
type PatternError struct {
	Value   string
	Pattern string
}

// Error formats the error using the sentinel prefix plus the pattern
// source.
func (e *PatternError) Error() string {
	return fmt.Sprintf("%s (pattern: %s)", errPatternMismatch, e.Pattern)
}

// Is bridges the typed error to the sentinel.
func (e *PatternError) Is(target error) bool {
	return target == errPatternMismatch
}

// Pattern fails when the wrapped string does not match the configured
// regular expression. The regex is expected to be compiled by the caller
// (e.g. via regexp.MustCompile) so a single Pattern can be reused across
// many values without recompiling.
type Pattern struct {
	value string
	re    *regexp.Regexp
}

// NewPattern wraps a string with a regular expression constraint.
// Passing nil is treated as "always fails" — MatchString on nil panics,
// but we convert that into an explicit *PatternError to keep the call
// shape uniform with the other validators in this package.
func NewPattern(value string, re *regexp.Regexp) *Pattern {
	return &Pattern{value: value, re: re}
}

// Value returns the wrapped string.
func (p *Pattern) Value() string { return p.value }

// Pattern returns the underlying compiled regex.
func (p *Pattern) Pattern() *regexp.Regexp { return p.re }

// Validate returns a *PatternError when the regex does not match value.
func (p *Pattern) Validate() error {
	if p.re == nil || !p.re.MatchString(p.value) {
		pat := "<nil>"
		if p.re != nil {
			pat = p.re.String()
		}
		return &PatternError{Value: p.value, Pattern: pat}
	}
	return nil
}
