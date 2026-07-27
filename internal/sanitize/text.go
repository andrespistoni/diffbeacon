// Package sanitize renders untrusted text without terminal control bytes.
package sanitize

import (
	"fmt"
	"strings"
)

// Text makes every C0, DEL, C1, ESC, CSI, and OSC control visible. Since ESC,
// the 8-bit CSI/OSC bytes, and their terminators are encoded independently, no
// terminal control sequence can survive the transformation.
func Text(value string) string {
	var result strings.Builder
	for _, character := range value {
		if character >= 0x20 && (character < 0x7f || character > 0x9f) {
			result.WriteRune(character)
			continue
		}
		if character <= 0x7f {
			fmt.Fprintf(&result, "\\x%02x", character)
		} else {
			fmt.Fprintf(&result, "\\u%04x", character)
		}
	}
	return result.String()
}

// DisplayText is Text with a readable tab exception for content already inside
// the managed TUI. All other terminal controls remain encoded.
func DisplayText(value string) string {
	return strings.ReplaceAll(Text(value), `\x09`, "\t")
}
