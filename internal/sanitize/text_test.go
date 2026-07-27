package sanitize

import (
	"strings"
	"testing"
)

func TestTextNeutralizesAllTerminalControlClasses(t *testing.T) {
	input := "c0\x00\n\tdel\x7f c1\u0085 csi\x1b[31m\u009b32m osc\x1b]52;c;payload\x07\u009dtitle\u009c"
	got := Text(input)
	for _, control := range []rune{'\x00', '\n', '\t', '\x7f', '\u0085', '\x1b', '\u009b', '\x07', '\u009d', '\u009c'} {
		if strings.ContainsRune(got, control) {
			t.Fatalf("Text() retained control U+%04X in %q", control, got)
		}
	}
	for _, visible := range []string{`\x00`, `\x0a`, `\x09`, `\x7f`, `\u0085`, `\x1b`, `\u009b`, `\u009d`, `\u009c`} {
		if !strings.Contains(got, visible) {
			t.Fatalf("Text() = %q, want %q", got, visible)
		}
	}
}

func TestDisplayTextOnlyPreservesTabs(t *testing.T) {
	got := DisplayText("a\tb\n\x1b[31m")
	if got != "a\tb\\x0a\\x1b[31m" {
		t.Fatalf("DisplayText() = %q", got)
	}
}
