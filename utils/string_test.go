package utils

import (
	"strings"
	"testing"
)

func TestStringPointer(t *testing.T) {
	s := "test-string"
	p := StringPointer(s)
	if p == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *p != s {
		t.Errorf("expected %q, got %q", s, *p)
	}
}

func TestStringWithCharset(t *testing.T) {
	length := 15
	customCharset := "ABC"
	result := StringWithCharset(length, customCharset)

	if len(result) != length {
		t.Errorf("expected length %d, got %d", length, len(result))
	}

	for _, char := range result {
		if !strings.ContainsRune(customCharset, char) {
			t.Errorf("found character %q outside of custom charset %q", char, customCharset)
		}
	}
}

func TestString(t *testing.T) {
	length := 10
	result := String(length)

	if len(result) != length {
		t.Errorf("expected length %d, got %d", length, len(result))
	}
}
