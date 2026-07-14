package string

import (
	"errors"
	"fmt"
	"net/mail"
)

// errEmailFailed is the sentinel returned by Email when the wrapped
// string is not a syntactically valid email address.
var errEmailFailed = errors.New("validator/string: value is not a valid email")

// EmailError is the typed error returned by Email. It carries the
// wrapped value and the underlying mail.ParseAddress error so callers
// can inspect both via errors.As and errors.Is/As chaining.
type EmailError struct {
	Value string
	Cause error
}

// Error formats the error using the sentinel prefix plus the quoted
// value and, when present, the underlying parser error.
func (e *EmailError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %q (%v)", errEmailFailed, e.Value, e.Cause)
	}
	return fmt.Sprintf("%s: %q", errEmailFailed, e.Value)
}

// Is bridges the typed error to the sentinel.
func (e *EmailError) Is(target error) bool {
	return target == errEmailFailed
}

// Unwrap exposes the underlying mail.ParseAddress error so callers can
// use errors.Is / errors.As against stdlib mail sentinels.
func (e *EmailError) Unwrap() error { return e.Cause }

// Email fails when the wrapped string cannot be parsed as an RFC 5322
// address by net/mail.ParseAddress. The check is intentionally lax —
// it does not verify that the domain actually accepts mail. Pair with
// a Pattern validator if a stricter shape is required.
type Email struct {
	value string
}

// NewEmail wraps a string in an Email validator.
func NewEmail(value string) *Email {
	return &Email{value: value}
}

// Value returns the wrapped string.
func (e *Email) Value() string { return e.value }

// Validate returns an *EmailError when mail.ParseAddress rejects the
// wrapped string.
func (e *Email) Validate() error {
	if _, err := mail.ParseAddress(e.value); err != nil {
		return &EmailError{Value: e.value, Cause: err}
	}
	return nil
}
