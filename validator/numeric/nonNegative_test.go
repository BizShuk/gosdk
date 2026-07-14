package numeric

import (
	"errors"
	"testing"
)

func TestNonNegative_PassZero(t *testing.T) {
	// Zero is non-negative.
	n := NewNonNegative(0)
	if err := n.Validate(); err != nil {
		t.Fatalf("zero should pass, got %v", err)
	}
}

func TestNonNegative_PassPositive(t *testing.T) {
	n := NewNonNegative(7)
	if err := n.Validate(); err != nil {
		t.Fatalf("positive should pass, got %v", err)
	}
}

func TestNonNegative_FailNegative(t *testing.T) {
	n := NewNonNegative(-1)
	err := n.Validate()
	if err == nil {
		t.Fatal("expected error for negative, got nil")
	}
	if !errors.Is(err, errNotNonNegative) {
		t.Fatalf("expected errNotNonNegative, got %v", err)
	}
}

func TestNonNegative_TypedError(t *testing.T) {
	n := NewNonNegative(-1)
	err := n.Validate()

	var typed *NonNegativeError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *NonNegativeError, got %T", err)
	}
	if typed.Value != -1 {
		t.Fatalf("Value = %d, want -1", typed.Value)
	}
	if !errors.Is(err, errNotNonNegative) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestNonNegative_Accessor(t *testing.T) {
	n := NewNonNegative(3)
	if got := n.Value(); got != 3 {
		t.Fatalf("Value() = %d, want 3", got)
	}
}
