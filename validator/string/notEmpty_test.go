package string

import (
	"errors"
	"testing"
)

func TestNotEmpty_Pass(t *testing.T) {
	n := NewNotEmpty("hello")
	if err := n.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNotEmpty_PassWhitespaceOnly(t *testing.T) {
	// Documented behaviour: " " is not empty. Pair with MinLen if you
	// need to reject whitespace-only inputs.
	n := NewNotEmpty(" ")
	if err := n.Validate(); err != nil {
		t.Fatalf("whitespace-only should still pass, got %v", err)
	}
}

func TestNotEmpty_Fail(t *testing.T) {
	n := NewNotEmpty("")
	err := n.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errNotEmptyFailed) {
		t.Fatalf("expected errNotEmptyFailed, got %v", err)
	}
}

func TestNotEmpty_TypedError(t *testing.T) {
	n := NewNotEmpty("")
	err := n.Validate()

	var typed *NotEmptyError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *NotEmptyError, got %T (%v)", err, err)
	}
	if typed.Value != "" {
		t.Fatalf("Value = %q, want %q", typed.Value, "")
	}
	// errors.Is must still bridge to the sentinel after the refactor.
	if !errors.Is(err, errNotEmptyFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestNotEmpty_ValueAccessor(t *testing.T) {
	n := NewNotEmpty("hello")
	if got := n.Value(); got != "hello" {
		t.Fatalf("Value() = %q, want %q", got, "hello")
	}
}
