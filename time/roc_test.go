package time

import (
	"testing"
)

func TestParseROCDate(t *testing.T) {
	res := ParseROCDate("112/05/20")
	if res.Year() != 2023 || res.Month() != 5 || res.Day() != 20 {
		t.Errorf("Expected 2023-05-20, got %v", res)
	}

	resErr := ParseROCDate("invalid")
	if !resErr.IsZero() {
		t.Errorf("Expected zero time for invalid date, got %v", resErr)
	}
}
