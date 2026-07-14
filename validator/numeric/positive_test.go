package numeric

import (
	"errors"
	"testing"
)

func TestPositive_Pass(t *testing.T) {
	p := NewPositive(5)
	if err := p.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestPositive_FailZero(t *testing.T) {
	p := NewPositive(0)
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for zero, got nil")
	}
	if !errors.Is(err, errNotPositive) {
		t.Fatalf("expected errNotPositive, got %v", err)
	}
}

func TestPositive_FailNegative(t *testing.T) {
	p := NewPositive(-1)
	if err := p.Validate(); !errors.Is(err, errNotPositive) {
		t.Fatalf("expected errNotPositive for negative, got %v", err)
	}
}

func TestPositive_TypedError(t *testing.T) {
	p := NewPositive(0)
	err := p.Validate()

	var typed *PositiveError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *PositiveError, got %T", err)
	}
	if typed.Value != 0 {
		t.Fatalf("Value = %d, want 0", typed.Value)
	}
	if !errors.Is(err, errNotPositive) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestPositive_Accessor(t *testing.T) {
	p := NewPositive(42)
	if got := p.Value(); got != 42 {
		t.Fatalf("Value() = %d, want 42", got)
	}
}
