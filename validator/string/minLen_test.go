package string

import (
	"errors"
	"strings"
	"testing"
)

func TestMinLen_Pass(t *testing.T) {
	m := NewMinLen("abcdef", 3)
	if err := m.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMinLen_PassAtBoundary(t *testing.T) {
	// len(value) == min must pass (>=).
	m := NewMinLen("abc", 3)
	if err := m.Validate(); err != nil {
		t.Fatalf("boundary length should pass, got %v", err)
	}
}

func TestMinLen_Fail(t *testing.T) {
	m := NewMinLen("ab", 5)
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errMinLenFailed) {
		t.Fatalf("expected errMinLenFailed, got %v", err)
	}
}

func TestMinLen_ErrorMentionsLengths(t *testing.T) {
	m := NewMinLen("ab", 5) // got=2, min=5
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "2") || !strings.Contains(msg, "5") {
		t.Fatalf("error must mention both lengths, got %q", msg)
	}
}

func TestMinLen_NonPositiveMinIsPermissive(t *testing.T) {
	m := NewMinLen("", 0)
	if err := m.Validate(); err != nil {
		t.Fatalf("min=0 should pass for empty string, got %v", err)
	}
}

func TestMinLen_TypedError(t *testing.T) {
	m := NewMinLen("ab", 5) // got=2, min=5
	err := m.Validate()

	var typed *MinLenError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *MinLenError, got %T", err)
	}
	if typed.Value != "ab" || typed.Min != 5 || typed.Got != 2 {
		t.Fatalf("got %+v, want {Value: ab, Min: 5, Got: 2}", typed)
	}
	if !errors.Is(err, errMinLenFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestMinLen_Accessors(t *testing.T) {
	m := NewMinLen("abc", 2)
	if got := m.Value(); got != "abc" {
		t.Fatalf("Value() = %q, want %q", got, "abc")
	}
	if got := m.Min(); got != 2 {
		t.Fatalf("Min() = %d, want 2", got)
	}
}
