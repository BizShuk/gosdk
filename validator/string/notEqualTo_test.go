package string

import (
	"errors"
	"strings"
	"testing"
)

func TestNotEqualTo_Pass(t *testing.T) {
	n := NewNotEqualTo("hello", "world")
	if err := n.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNotEqualTo_Fail(t *testing.T) {
	n := NewNotEqualTo("hello", "hello")
	err := n.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errNotEqualToFailed) {
		t.Fatalf("expected errNotEqualToFailed, got %v", err)
	}
}

func TestNotEqualTo_FailBothEmpty(t *testing.T) {
	// Two empty strings are equal — must fail.
	n := NewNotEqualTo("", "")
	if err := n.Validate(); !errors.Is(err, errNotEqualToFailed) {
		t.Fatalf("expected errNotEqualToFailed, got %v", err)
	}
}

func TestNotEqualTo_TypedError(t *testing.T) {
	n := NewNotEqualTo("hello", "hello")
	err := n.Validate()

	var typed *NotEqualToError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *NotEqualToError, got %T", err)
	}
	if typed.Value != "hello" || typed.Forbidden != "hello" {
		t.Fatalf("got %+v, want {Value: hello, Forbidden: hello}", typed)
	}
	if !errors.Is(err, errNotEqualToFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestNotEqualTo_ErrorMentionsValues(t *testing.T) {
	n := NewNotEqualTo("hello", "hello")
	err := n.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	// "hello" appears twice in the message; just confirm at least once.
	if !strings.Contains(msg, "hello") {
		t.Fatalf("error must mention the forbidden value, got %q", msg)
	}
}

func TestNotEqualTo_Accessors(t *testing.T) {
	n := NewNotEqualTo("a", "b")
	if got := n.Value(); got != "a" {
		t.Fatalf("Value() = %q, want %q", got, "a")
	}
	if got := n.Forbidden(); got != "b" {
		t.Fatalf("Forbidden() = %q, want %q", got, "b")
	}
}
