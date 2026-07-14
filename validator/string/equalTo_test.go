package string

import (
	"errors"
	"strings"
	"testing"
)

func TestEqualTo_Pass(t *testing.T) {
	e := NewEqualTo("hello", "hello")
	if err := e.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestEqualTo_Fail(t *testing.T) {
	e := NewEqualTo("hello", "world")
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errEqualToFailed) {
		t.Fatalf("expected errEqualToFailed, got %v", err)
	}
}

func TestEqualTo_FailEmpty(t *testing.T) {
	// Empty value must not equal non-empty expected (and vice-versa).
	e := NewEqualTo("", "x")
	if err := e.Validate(); !errors.Is(err, errEqualToFailed) {
		t.Fatalf("expected errEqualToFailed for empty vs non-empty, got %v", err)
	}
}

func TestEqualTo_TypedError(t *testing.T) {
	e := NewEqualTo("hello", "world")
	err := e.Validate()

	var typed *EqualToError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *EqualToError, got %T", err)
	}
	if typed.Value != "hello" || typed.Expected != "world" {
		t.Fatalf("got %+v, want {Value: hello, Expected: world}", typed)
	}
	if !errors.Is(err, errEqualToFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestEqualTo_ErrorMentionsValues(t *testing.T) {
	e := NewEqualTo("hello", "world")
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "hello") || !strings.Contains(msg, "world") {
		t.Fatalf("error must mention both values, got %q", msg)
	}
}

func TestEqualTo_CaseSensitive(t *testing.T) {
	// Documented behaviour: byte-exact comparison means "Hello" != "hello".
	e := NewEqualTo("Hello", "hello")
	if err := e.Validate(); !errors.Is(err, errEqualToFailed) {
		t.Fatalf("expected errEqualToFailed, got %v", err)
	}
}

func TestEqualTo_Accessors(t *testing.T) {
	e := NewEqualTo("a", "b")
	if got := e.Value(); got != "a" {
		t.Fatalf("Value() = %q, want %q", got, "a")
	}
	if got := e.Expected(); got != "b" {
		t.Fatalf("Expected() = %q, want %q", got, "b")
	}
}
