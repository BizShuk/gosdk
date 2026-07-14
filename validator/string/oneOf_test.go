package string

import (
	"errors"
	"testing"
)

func TestOneOf_Pass(t *testing.T) {
	o := NewOneOf("apple", "apple", "banana", "cherry")
	if err := o.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestOneOf_Fail(t *testing.T) {
	o := NewOneOf("grape", "apple", "banana", "cherry")
	err := o.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errOneOfFailed) {
		t.Fatalf("expected errOneOfFailed, got %v", err)
	}
}

func TestOneOf_EmptyAlwaysFails(t *testing.T) {
	// An empty allowed set is a vacuous constraint: every value fails.
	o := NewOneOf("anything")
	if err := o.Validate(); !errors.Is(err, errOneOfFailed) {
		t.Fatalf("expected errOneOfFailed for empty allowed set, got %v", err)
	}
}

func TestOneOf_DefensiveCopy(t *testing.T) {
	in := []string{"a", "b"}
	o := NewOneOf("c", in...)
	// Mutating the caller's slice must not change the validator's
	// behaviour: "c" is not in {a, b}, so it must still fail.
	in[0] = "c"
	if err := o.Validate(); !errors.Is(err, errOneOfFailed) {
		t.Fatalf("expected errOneOfFailed after caller mutation, got %v", err)
	}
}

func TestOneOf_TypedError(t *testing.T) {
	o := NewOneOf("grape", "apple", "banana")
	err := o.Validate()

	var typed *OneOfError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *OneOfError, got %T", err)
	}
	if typed.Value != "grape" {
		t.Fatalf("Value = %q, want %q", typed.Value, "grape")
	}
	if len(typed.Allowed) != 2 || typed.Allowed[0] != "apple" || typed.Allowed[1] != "banana" {
		t.Fatalf("Allowed = %v, want [apple banana]", typed.Allowed)
	}
	if !errors.Is(err, errOneOfFailed) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestOneOf_TypedErrorAllowedIsCopy(t *testing.T) {
	// The Allowed slice on the error must be a defensive copy so the
	// caller cannot mutate validator state through it.
	o := NewOneOf("grape", "apple", "banana")
	err := o.Validate()
	var typed *OneOfError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *OneOfError")
	}
	typed.Allowed[0] = "MUTATED"
	if err := o.Validate(); !errors.Is(err, errOneOfFailed) {
		t.Fatal("mutating typed.Allowed must not affect the validator")
	}
}

func TestOneOf_Accessors(t *testing.T) {
	o := NewOneOf("apple", "apple", "banana")
	if got := o.Value(); got != "apple" {
		t.Fatalf("Value() = %q, want %q", got, "apple")
	}
	allowed := o.Allowed()
	if len(allowed) != 2 || allowed[0] != "apple" || allowed[1] != "banana" {
		t.Fatalf("Allowed() = %v, want [apple banana]", allowed)
	}
	// Allowed() must return a defensive copy.
	allowed[0] = "MUTATED"
	if err := o.Validate(); err != nil {
		t.Fatalf("Allowed() must return a copy, got %v", err)
	}
}

func TestOneOf_CaseSensitive(t *testing.T) {
	// Documented behaviour: "Apple" must not match "apple".
	o := NewOneOf("Apple", "apple")
	if err := o.Validate(); !errors.Is(err, errOneOfFailed) {
		t.Fatalf("expected errOneOfFailed (case mismatch), got %v", err)
	}
}
