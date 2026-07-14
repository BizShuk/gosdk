package string

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestPattern_Pass(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	p := NewPattern("hello", re)
	if err := p.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestPattern_Fail(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	// Capital "H" violates ^[a-z]+$.
	p := NewPattern("Hello", re)
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errPatternMismatch) {
		t.Fatalf("expected errPatternMismatch, got %v", err)
	}
}

func TestPattern_ErrorMentionsPattern(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	p := NewPattern("Hello", re)
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "[a-z]+") {
		t.Fatalf("error must mention pattern source, got %q", err.Error())
	}
}

func TestPattern_NilRegexDoesNotPanic(t *testing.T) {
	// nil *regexp.Regexp must surface as errPatternMismatch, not panic.
	p := NewPattern("anything", nil)
	if err := p.Validate(); !errors.Is(err, errPatternMismatch) {
		t.Fatalf("expected errPatternMismatch, got %v", err)
	}
}

func TestPattern_TypedError(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	p := NewPattern("Hello", re)
	err := p.Validate()

	var typed *PatternError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *PatternError, got %T", err)
	}
	if typed.Value != "Hello" || typed.Pattern != "^[a-z]+$" {
		t.Fatalf("got %+v, want {Value: Hello, Pattern: ^[a-z]+$}", typed)
	}
	if !errors.Is(err, errPatternMismatch) {
		t.Fatal("typed error must satisfy errors.Is against the sentinel")
	}
}

func TestPattern_TypedErrorOnNilRegex(t *testing.T) {
	// nil regex must still produce *PatternError, not panic.
	p := NewPattern("anything", nil)
	err := p.Validate()
	var typed *PatternError
	if !errors.As(err, &typed) {
		t.Fatalf("expected *PatternError, got %T", err)
	}
	if typed.Pattern != "<nil>" {
		t.Fatalf("Pattern = %q, want %q", typed.Pattern, "<nil>")
	}
}

func TestPattern_Accessors(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	p := NewPattern("hi", re)
	if got := p.Value(); got != "hi" {
		t.Fatalf("Value() = %q, want %q", got, "hi")
	}
	if p.Pattern() != re {
		t.Fatal("Pattern() must return the same compiled regex")
	}
}
