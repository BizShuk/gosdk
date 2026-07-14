package numeric

import (
	"errors"
	"strings"
	"testing"
)

func TestRange_Pass(t *testing.T) {
	r := NewRange(5, 1, 10)
	if err := r.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRange_PassAtBoundaries(t *testing.T) {
	// Both endpoints are inclusive.
	if err := NewRange(1, 1, 10).Validate(); err != nil {
		t.Fatalf("lower boundary should pass, got %v", err)
	}
	if err := NewRange(10, 1, 10).Validate(); err != nil {
		t.Fatalf("upper boundary should pass, got %v", err)
	}
}

func TestRange_FailBelow(t *testing.T) {
	r := NewRange(0, 1, 10)
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errRangeFailed) {
		t.Fatalf("expected errRangeFailed, got %v", err)
	}
}

func TestRange_FailAbove(t *testing.T) {
	r := NewRange(11, 1, 10)
	if err := r.Validate(); !errors.Is(err, errRangeFailed) {
		t.Fatalf("expected errRangeFailed, got %v", err)
	}
}

func TestRange_ErrorMentionsBounds(t *testing.T) {
	r := NewRange(0, 1, 10) // value=0, range=[1,10]
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"0", "1", "10"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error must mention %q, got %q", want, msg)
		}
	}
}

func TestRange_TypedError(t *testing.T) {
	r := NewRange(0, 1, 10)
	err := r.Validate()

	var typed *RangeError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *RangeError, got %T", err)
	}
	if typed.Value != 0 || typed.Min != 1 || typed.Max != 10 {
		t.Fatalf("got %+v, want {Value: 0, Min: 1, Max: 10}", typed)
	}
	if !errors.Is(err, errRangeFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestRange_Accessors(t *testing.T) {
	r := NewRange(5, 2, 8)
	if got := r.Value(); got != 5 {
		t.Fatalf("Value() = %d, want 5", got)
	}
	if got := r.Min(); got != 2 {
		t.Fatalf("Min() = %d, want 2", got)
	}
	if got := r.Max(); got != 8 {
		t.Fatalf("Max() = %d, want 8", got)
	}
}
