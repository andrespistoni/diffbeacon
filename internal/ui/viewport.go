package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func viewportLines(lines []string, vertical, horizontal, width, height int) []string {
	if len(lines) == 0 || width <= 0 || height <= 0 {
		return nil
	}
	vertical = min(max(0, vertical), len(lines)-1)
	end := min(len(lines), vertical+height)
	result := make([]string, 0, end-vertical)
	for _, line := range lines[vertical:end] {
		result = append(result, horizontalSlice(line, horizontal, width))
	}
	return result
}

func horizontalSlice(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	offset = max(0, offset)
	if offset >= ansi.StringWidth(value) {
		return ""
	}
	return ansi.Cut(value, offset, offset+width)
}

func padCell(value string, width int) string {
	value = horizontalSlice(value, 0, width)
	if padding := width - ansi.StringWidth(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func wrapLines(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, strings.Split(ansi.Wordwrap(line, width, " "), "\n")...)
	}
	return result
}

func fitWidthLines(value string, width, height int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = horizontalSlice(lines[index], 0, width)
	}
	return strings.Join(lines, "\n")
}
