package string

import (
	"errors"
	"strings"
	"testing"
)

func TestEmail_Pass(t *testing.T) {
	e := NewEmail("user@example.com")
	if err := e.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestEmail_PassPlusTag(t *testing.T) {
	// Plus-tag addressing should still parse.
	e := NewEmail("user+tag@example.com")
	if err := e.Validate(); err != nil {
		t.Fatalf("plus-tagged email should pass, got %v", err)
	}
}

func TestEmail_Fail(t *testing.T) {
	e := NewEmail("not_an_email")
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errEmailFailed) {
		t.Fatalf("expected errEmailFailed, got %v", err)
	}
}

func TestEmail_FailEmpty(t *testing.T) {
	e := NewEmail("")
	if err := e.Validate(); !errors.Is(err, errEmailFailed) {
		t.Fatalf("expected errEmailFailed for empty, got %v", err)
	}
}

func TestEmail_ErrorQuotesValue(t *testing.T) {
	e := NewEmail("not_an_email")
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not_an_email") {
		t.Fatalf("error must quote the bad input, got %q", err.Error())
	}
}

func TestEmail_TypedError(t *testing.T) {
	e := NewEmail("not_an_email")
	err := e.Validate()

	var typed *EmailError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *EmailError, got %T", err)
	}
	if typed.Value != "not_an_email" {
		t.Fatalf("Value = %q, want %q", typed.Value, "not_an_email")
	}
	if typed.Cause == nil {
		t.Fatal("Cause must be set to the underlying mail.ParseAddress error")
	}
	if !errors.Is(err, errEmailFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestEmail_Accessor(t *testing.T) {
	e := NewEmail("a@b.co")
	if got := e.Value(); got != "a@b.co" {
		t.Fatalf("Value() = %q, want %q", got, "a@b.co")
	}
}
