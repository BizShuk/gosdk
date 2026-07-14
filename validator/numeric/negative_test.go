package numeric

import (
	"errors"
	"testing"
)

func TestNegative_Pass(t *testing.T) {
	n := NewNegative(-5)
	if err := n.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNegative_FailZero(t *testing.T) {
	n := NewNegative(0)
	err := n.Validate()
	if err == nil {
		t.Fatal("expected error for zero, got nil")
	}
	if !errors.Is(err, errNotNegative) {
		t.Fatalf("expected errNotNegative, got %v", err)
	}
}

func TestNegative_FailPositive(t *testing.T) {
	n := NewNegative(3)
	if err := n.Validate(); !errors.Is(err, errNotNegative) {
		t.Fatalf("expected errNotNegative for positive, got %v", err)
	}
}

func TestNegative_TypedError(t *testing.T) {
	n := NewNegative(5)
	err := n.Validate()

	var typed *NegativeError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *NegativeError, got %T", err)
	}
	if typed.Value != 5 {
		t.Fatalf("Value = %d, want 5", typed.Value)
	}
	if !errors.Is(err, errNotNegative) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestNegative_Accessor(t *testing.T) {
	n := NewNegative(-2)
	if got := n.Value(); got != -2 {
		t.Fatalf("Value() = %d, want -2", got)
	}
}
