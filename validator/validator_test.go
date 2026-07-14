package validator

import (
	"errors"
	"testing"
)

// stubValidator is a deterministic IValidator used by the composite tests.
type stubValidator struct {
	err error
}

func (s stubValidator) Validate() error { return s.err }

func TestNew_Empty(t *testing.T) {
	// A composite with no children is trivially valid.
	v := New()
	if err := v.Validate(); err != nil {
		t.Fatalf("empty composite should pass, got %v", err)
	}
}

func TestValidator_AllPass(t *testing.T) {
	v := New(stubValidator{nil}, stubValidator{nil})
	if err := v.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidator_FirstFailureWins(t *testing.T) {
	boom := errors.New("boom")
	v := New(stubValidator{nil}, stubValidator{boom}, stubValidator{errors.New("ignored")})
	err := v.Validate()
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestValidator_NestedStopsAtChild(t *testing.T) {
	boom := errors.New("boom")
	inner := New(stubValidator{nil}, stubValidator{boom})
	outer := New(stubValidator{nil}, inner, stubValidator{errors.New("ignored")})
	err := outer.Validate()
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestValidator_Add_Appends(t *testing.T) {
	boom := errors.New("boom")
	v := New()
	v.Add(stubValidator{nil})
	v.Add(stubValidator{boom})
	if err := v.Validate(); !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestValidator_Add_Chains(t *testing.T) {
	v := New()
	if got := v.Add(stubValidator{}); got != v {
		t.Fatalf("Add should return the receiver for chaining, got %p want %p", got, v)
	}
}

func TestValidator_DefensiveCopyOnConstruct(t *testing.T) {
	boom := errors.New("boom")
	in := []IValidator{stubValidator{nil}}
	v := New(in...)
	// Mutating the caller's slice after construction must not affect the
	// composite.
	in[0] = stubValidator{boom}
	if err := v.Validate(); err != nil {
		t.Fatalf("composite must hold its own copy, got %v", err)
	}
}

func TestValidator_RecursesDeeply(t *testing.T) {
	deep := errors.New("deep")
	leaf := stubValidator{deep}
	// Chain four nested composites; the deep error must still surface.
	v := New(
		New(stubValidator{nil}),
		New(stubValidator{nil}, New(stubValidator{nil}, leaf)),
	)
	if err := v.Validate(); !errors.Is(err, deep) {
		t.Fatalf("expected deep, got %v", err)
	}
}
