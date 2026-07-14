package string

import (
	"errors"
	"fmt"
	"net/url"
)

// errURLFailed is the sentinel returned by URL when the wrapped string
// cannot be parsed as an absolute URL.
var errURLFailed = errors.New("validator/string: value is not a valid URL")

// URLError is the typed error returned by URL. It carries the wrapped
// value, a short human-readable Reason ("parse error" or "missing
// scheme or host"), and, when applicable, the underlying url.Parse
// error.
type URLError struct {
	Value  string
	Reason string
	Cause  error
}

// Error formats the error using the sentinel prefix plus the reason
// and, when present, the underlying parser error.
func (e *URLError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %q (%v)", errURLFailed, e.Reason, e.Value, e.Cause)
	}
	return fmt.Sprintf("%s (%s): %q", errURLFailed, e.Reason, e.Value)
}

// Is bridges the typed error to the sentinel.
func (e *URLError) Is(target error) bool {
	return target == errURLFailed
}

// Unwrap exposes the underlying url.Parse error so errors.Is can chain
// through to net/url sentinels (e.g. url.EscapeError).
func (e *URLError) Unwrap() error { return e.Cause }

// URL fails when the wrapped string is not a syntactically valid
// absolute URL — i.e. url.Parse succeeds AND a non-empty scheme AND
// a non-empty host are present. Relative URLs ("/foo", "bar/baz")
// intentionally do not pass; pair with a Pattern validator if relative
// references must also be accepted.
type URL struct {
	value string
}

// NewURL wraps a string in a URL validator.
func NewURL(value string) *URL {
	return &URL{value: value}
}

// Value returns the wrapped string.
func (u *URL) Value() string { return u.value }

// Validate returns a *URLError when url.Parse rejects the wrapped
// string, or when the parseable result is missing scheme or host.
func (u *URL) Validate() error {
	parsed, err := url.Parse(u.value)
	if err != nil {
		return &URLError{Value: u.value, Reason: "parse error", Cause: err}
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return &URLError{Value: u.value, Reason: "missing scheme or host"}
	}
	return nil
}
