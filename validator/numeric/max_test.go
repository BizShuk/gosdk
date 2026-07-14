package numeric

import (
	"errors"
	"strings"
	"testing"
)

func TestMax_Pass(t *testing.T) {
	m := NewMax(3, 5)
	if err := m.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMax_PassAtBoundary(t *testing.T) {
	// value == max must pass.
	m := NewMax(5, 5)
	if err := m.Validate(); err != nil {
		t.Fatalf("boundary should pass, got %v", err)
	}
}

func TestMax_Fail(t *testing.T) {
	m := NewMax(10, 5)
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errMaxFailed) {
		t.Fatalf("expected errMaxFailed, got %v", err)
	}
}

func TestMax_ErrorMentionsValues(t *testing.T) {
	m := NewMax(10, 5) // actual=10, want<=5
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "10") || !strings.Contains(msg, "5") {
		t.Fatalf("error must mention actual and required, got %q", msg)
	}
}

func TestMax_TypedError(t *testing.T) {
	m := NewMax(10, 5)
	err := m.Validate()

	var typed *MaxError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *MaxError, got %T", err)
	}
	if typed.Value != 10 || typed.Max != 5 {
		t.Fatalf("got %+v, want {Value: 10, Max: 5}", typed)
	}
	if !errors.Is(err, errMaxFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestMax_Accessors(t *testing.T) {
	m := NewMax(3, 9)
	if got := m.Value(); got != 3 {
		t.Fatalf("Value() = %d, want 3", got)
	}
	if got := m.Max(); got != 9 {
		t.Fatalf("Max() = %d, want 9", got)
	}
}
