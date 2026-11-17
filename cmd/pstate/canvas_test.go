package pstate

import (
	"testing"
)

func plainPaint(_ paint, s string) string { return s }

func TestCanvasJoinsLines(t *testing.T) {
	c := newCanvas(5, 3)
	c.hline(0, 2, 1, false) // ─ ─ ┬
	c.vline(2, 1, 2, false) //     │
	c.hline(2, 4, 1, false)
	got := c.render(0, 0, 5, 3, plainPaint)
	want := []string{"     ", "──┬──", "  │  "}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: %q want %q", i, got[i], want[i])
		}
	}
}

func TestCanvasViewportAndWideRunes(t *testing.T) {
	c := newCanvas(6, 1)
	c.text(0, 0, "a前b", 6, paint{})
	if got := c.render(0, 0, 6, 1, plainPaint)[0]; got != "a前b  " {
		t.Fatalf("full: %q", got)
	}
	if got := c.render(2, 0, 4, 1, plainPaint)[0]; got != " b  " {
		t.Fatalf("viewport starting on the second half of a wide rune: %q", got)
	}
	if got := c.render(0, 0, 2, 1, plainPaint)[0]; got != "a " {
		t.Fatalf("wide rune cut at the right edge: %q", got)
	}
	if got := c.render(4, 0, 4, 1, plainPaint)[0]; textWidth(got) != 4 {
		t.Fatalf("viewport past the canvas must still be padded: %q", got)
	}
}
