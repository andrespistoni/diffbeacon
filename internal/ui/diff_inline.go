package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
)

func renderInline(state app.Model, styles styles) []string {
	model := state.Detail.Diff
	if model == nil {
		return nil
	}
	if model.Degraded {
		return []string{"! View degraded: " + nonEmpty(model.Reason, "text diff unavailable")}
	}
	if (state.Density == app.DensityChanges || !model.Document.Capability.FullFile) && !selectedUntracked(state) {
		return renderChanges(*model, state.ActiveHunkID, styles)
	}
	return renderFull(*model, state.ActiveHunkID, styles)
}

func renderChanges(model diffpkg.Model, active string, styles styles) []string {
	var result []string
	for _, hunk := range model.Hunks {
		marker := "  "
		if hunk.ID == active {
			marker = "▶ "
		}
		header := fmt.Sprintf("%s@@ -%d,%d +%d,%d @@ [%s]", marker, hunk.BeforeStart, hunk.BeforeLineCount, hunk.AfterStart, hunk.AfterLineCount, hunk.ID)
		result = append(result, styles.hunk.Render(header))
		for _, row := range hunk.Rows {
			result = append(result, renderRow(row, styles)...)
		}
	}
	return result
}

func renderFull(model diffpkg.Model, active string, styles styles) []string {
	result := make([]string, 0, len(model.FullRows))
	activeRows := make(map[diffpkg.Row]struct{})
	for _, hunk := range model.Hunks {
		if hunk.ID != active {
			continue
		}
		for _, row := range hunk.Rows {
			activeRows[row] = struct{}{}
		}
		break
	}
	for _, row := range model.FullRows {
		prefix := "  "
		if _, ok := activeRows[row]; ok {
			prefix = "▶ "
		}
		lines := renderRow(row, styles)
		for _, line := range lines {
			result = append(result, prefix+line)
			prefix = "  "
		}
	}
	return result
}

func renderScrollOffset(model diffpkg.Model, density app.Density, rowOffset int) int {
	if rowOffset <= 0 {
		return 0
	}
	rowsSeen, linesSeen := 0, 0
	if density == app.DensityChanges {
		for _, hunk := range model.Hunks {
			if rowsSeen == rowOffset {
				return linesSeen
			}
			linesSeen++ // hunk header
			for _, row := range hunk.Rows {
				if rowsSeen == rowOffset {
					return linesSeen
				}
				rowsSeen++
				linesSeen += renderedRowLineCount(row)
			}
		}
		return linesSeen
	}
	for _, row := range model.FullRows {
		if rowsSeen == rowOffset {
			return linesSeen
		}
		rowsSeen++
		linesSeen += renderedRowLineCount(row)
	}
	return linesSeen
}

func renderedRowLineCount(row diffpkg.Row) int {
	if row.Kind == diffpkg.RowChanged && row.Before != nil && row.After != nil {
		return 2
	}
	return 1
}

func renderRow(row diffpkg.Row, styles styles) []string {
	switch row.Kind {
	case diffpkg.RowEqual:
		return []string{formatDiffLine(row.Before, row.After, " ", renderLine(row.After, styles), styles)}
	case diffpkg.RowAdded:
		return []string{styles.addedLine.Render(formatDiffLine(nil, row.After, "+", renderLine(row.After, styles), styles))}
	case diffpkg.RowDeleted:
		return []string{styles.deletedLine.Render(formatDiffLine(row.Before, nil, "-", renderLine(row.Before, styles), styles))}
	case diffpkg.RowChanged:
		result := make([]string, 0, 2)
		if row.Before != nil {
			result = append(result, styles.deletedLine.Render(formatDiffLine(row.Before, nil, "-", renderLine(row.Before, styles), styles)))
		}
		if row.After != nil {
			result = append(result, styles.addedLine.Render(formatDiffLine(nil, row.After, "+", renderLine(row.After, styles), styles)))
		}
		return result
	default:
		return []string{"! unknown diff row"}
	}
}

func formatDiffLine(before, after *diffpkg.Line, mark, text string, styles styles) string {
	oldNumber, newNumber := "", ""
	if before != nil {
		oldNumber = strconv.Itoa(before.Number)
	}
	if after != nil {
		newNumber = strconv.Itoa(after.Number)
	}
	if mark == "+" {
		mark = styles.addedMark.Render(mark)
	} else if mark == "-" {
		mark = styles.deletedMark.Render(mark)
	}
	return fmt.Sprintf("%4s %4s %s %s", oldNumber, newNumber, mark, text)
}

func renderLine(line *diffpkg.Line, styles styles) string {
	if line == nil {
		return ""
	}
	if len(line.Spans) == 0 || !styles.color {
		return line.Text
	}
	var result strings.Builder
	for _, span := range line.Spans {
		style := tokenStyle(span.Style, styles)
		result.WriteString(style.Render(span.Text))
	}
	return result.String()
}

func tokenStyle(token string, styles styles) lipgloss.Style {
	base := lipgloss.NewStyle()
	switch {
	case strings.HasPrefix(token, "Keyword"):
		return base.Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7E22CE", Dark: "#D8B4FE"})
	case strings.HasPrefix(token, "Comment"):
		return styles.muted.Italic(true)
	case strings.HasPrefix(token, "LiteralString"):
		return base.Foreground(lipgloss.AdaptiveColor{Light: "#166534", Dark: "#86EFAC"})
	case strings.HasPrefix(token, "LiteralNumber"):
		return base.Foreground(lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"})
	case strings.HasPrefix(token, "NameFunction"), strings.HasPrefix(token, "NameClass"):
		return base.Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#FDBA74"})
	default:
		return base
	}
}

func rowInActiveHunk(row diffpkg.Row, hunks []diffpkg.Hunk, active string) bool {
	if active == "" {
		return false
	}
	for _, hunk := range hunks {
		if hunk.ID != active {
			continue
		}
		for _, candidate := range hunk.Rows {
			if candidate.Before == row.Before && candidate.After == row.After {
				return true
			}
		}
	}
	return false
}

func hunkOffset(model diffpkg.Model, index int, density app.Density) int {
	if index < 0 || index >= len(model.Hunks) {
		return 0
	}
	if density == app.DensityChanges {
		offset := 0
		for current := 0; current < index; current++ {
			offset += len(model.Hunks[current].Rows)
		}
		return offset
	}
	for offset, row := range model.FullRows {
		if rowInActiveHunk(row, model.Hunks, model.Hunks[index].ID) {
			return offset
		}
	}
	return 0
}
