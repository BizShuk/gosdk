package string

import (
	"errors"
	"testing"
)

func TestURL_Pass(t *testing.T) {
	u := NewURL("https://example.com/path?q=1")
	if err := u.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestURL_FailNoScheme(t *testing.T) {
	u := NewURL("example.com") // url.Parse accepts this, but scheme is empty
	err := u.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errURLFailed) {
		t.Fatalf("expected errURLFailed, got %v", err)
	}
}

func TestURL_FailRelativePath(t *testing.T) {
	u := NewURL("/foo/bar")
	if err := u.Validate(); !errors.Is(err, errURLFailed) {
		t.Fatalf("expected errURLFailed for relative path, got %v", err)
	}
}

func TestURL_FailEmpty(t *testing.T) {
	u := NewURL("")
	if err := u.Validate(); !errors.Is(err, errURLFailed) {
		t.Fatalf("expected errURLFailed for empty, got %v", err)
	}
}

func TestURL_TypedErrorMissingScheme(t *testing.T) {
	u := NewURL("example.com")
	err := u.Validate()

	var typed *URLError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *URLError, got %T", err)
	}
	if typed.Value != "example.com" {
		t.Fatalf("Value = %q, want %q", typed.Value, "example.com")
	}
	if typed.Reason != "missing scheme or host" {
		t.Fatalf("Reason = %q, want %q", typed.Reason, "missing scheme or host")
	}
	if typed.Cause != nil {
		t.Fatalf("Cause should be nil when parser succeeded but scheme/host are missing, got %v", typed.Cause)
	}
	if !errors.Is(err, errURLFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestURL_Accessor(t *testing.T) {
	u := NewURL("https://x.io")
	if got := u.Value(); got != "https://x.io" {
		t.Fatalf("Value() = %q, want %q", got, "https://x.io")
	}
}
