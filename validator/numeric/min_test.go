package numeric

import (
	"errors"
	"strings"
	"testing"
)

func TestMin_Pass(t *testing.T) {
	m := NewMin(10, 5)
	if err := m.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMin_PassAtBoundary(t *testing.T) {
	// value == min must pass.
	m := NewMin(5, 5)
	if err := m.Validate(); err != nil {
		t.Fatalf("boundary should pass, got %v", err)
	}
}

func TestMin_Fail(t *testing.T) {
	m := NewMin(3, 5)
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errMinFailed) {
		t.Fatalf("expected errMinFailed, got %v", err)
	}
}

func TestMin_ErrorMentionsValues(t *testing.T) {
	m := NewMin(3, 5) // actual=3, want>=5
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "3") || !strings.Contains(msg, "5") {
		t.Fatalf("error must mention actual and required, got %q", msg)
	}
}

func TestMin_TypedError(t *testing.T) {
	m := NewMin(3, 5)
	err := m.Validate()

	var typed *MinError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *MinError, got %T", err)
	}
	if typed.Value != 3 || typed.Min != 5 {
		t.Fatalf("got %+v, want {Value: 3, Min: 5}", typed)
	}
	if !errors.Is(err, errMinFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestMin_Accessors(t *testing.T) {
	m := NewMin(7, 2)
	if got := m.Value(); got != 7 {
		t.Fatalf("Value() = %d, want 7", got)
	}
	if got := m.Min(); got != 2 {
		t.Fatalf("Min() = %d, want 2", got)
	}
}
