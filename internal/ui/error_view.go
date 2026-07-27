package ui

import (
	"strings"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
)

func (m Model) visibleError() *app.AppError {
	if m.state.Detail.Error != nil {
		return m.state.Detail.Error
	}
	if m.state.Error != nil {
		return m.state.Error
	}
	if m.watchError != "" {
		return &app.AppError{Summary: "watcher failed", Detail: m.watchError}
	}
	return nil
}

func renderErrorView(problem *app.AppError, detail bool) []string {
	if problem == nil {
		return nil
	}
	lines := []string{"! " + diffpkg.SafeDisplayText(problem.Summary)}
	if !detail {
		return append(lines, "Press e for sanitized detail; the TUI remains active.")
	}
	lines = append(lines, "Error detail (press e or Esc to close):", "")
	for _, line := range strings.Split(problem.Detail, "\n") {
		lines = append(lines, diffpkg.SafeDisplayText(line))
	}
	return lines
}
