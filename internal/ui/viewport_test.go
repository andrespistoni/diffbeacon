package ui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestViewportClampsAndSlicesUnicodeAndANSI(t *testing.T) {
	lines := []string{"zero", "\x1b[31m一two-three\x1b[0m", "last"}
	got := viewportLines(lines, 1, 2, 5, 1)
	if len(got) != 1 || ansi.StringWidth(got[0]) > 5 || got[0] == "" {
		t.Fatalf("viewport = %#v, width = %d", got, ansi.StringWidth(got[0]))
	}
	if got := viewportLines(lines, 99, 0, 10, 2); len(got) != 1 || got[0] != "last" {
		t.Fatalf("clamped viewport = %#v", got)
	}
}

func TestViewportHorizontalOffsetIsSharedDeterministically(t *testing.T) {
	lines := []string{"abcdefgh", "ABCDEFGH"}
	first := viewportLines(lines, 0, 3, 3, 2)
	second := viewportLines(lines, 0, 3, 3, 2)
	if first[0] != "def" || first[1] != "DEF" || first[0] != second[0] || first[1] != second[1] {
		t.Fatalf("viewport first=%#v second=%#v", first, second)
	}
}
