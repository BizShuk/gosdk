package string

import (
	"errors"
	"strings"
	"testing"
)

func TestMaxLen_Pass(t *testing.T) {
	m := NewMaxLen("hello", 10)
	if err := m.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMaxLen_PassAtBoundary(t *testing.T) {
	// len(value) == max must pass (<=, not <).
	m := NewMaxLen("hello", 5)
	if err := m.Validate(); err != nil {
		t.Fatalf("boundary length should pass, got %v", err)
	}
}

func TestMaxLen_Fail(t *testing.T) {
	m := NewMaxLen("hello world", 5)
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errMaxLenFailed) {
		t.Fatalf("expected errMaxLenFailed, got %v", err)
	}
}

func TestMaxLen_ErrorMentionsLengths(t *testing.T) {
	m := NewMaxLen("hello world", 5) // got=11, max=5
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "11") || !strings.Contains(msg, "5") {
		t.Fatalf("error must mention actual (11) and max (5), got %q", msg)
	}
}

func TestMaxLen_TypedError(t *testing.T) {
	m := NewMaxLen("hello world", 5) // got=11, max=5
	err := m.Validate()

	var typed *MaxLenError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *MaxLenError, got %T", err)
	}
	if typed.Value != "hello world" || typed.Max != 5 || typed.Got != 11 {
		t.Fatalf("got %+v, want {Value: hello world, Max: 5, Got: 11}", typed)
	}
	if !errors.Is(err, errMaxLenFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestMaxLen_Accessors(t *testing.T) {
	m := NewMaxLen("hi", 5)
	if got := m.Value(); got != "hi" {
		t.Fatalf("Value() = %q, want %q", got, "hi")
	}
	if got := m.Max(); got != 5 {
		t.Fatalf("Max() = %d, want 5", got)
	}
}
