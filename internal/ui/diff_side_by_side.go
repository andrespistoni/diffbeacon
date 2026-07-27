package ui

import (
	"fmt"
	"strings"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
)

func renderSideBySide(state app.Model, styles styles, width int) []string {
	model := state.Detail.Diff
	if model == nil || model.Degraded || width < 3 {
		return nil
	}
	leftWidth := (width - 3) / 2
	rightWidth := width - 3 - leftWidth
	var result []string
	appendRow := func(row diffpkg.Row) {
		active := rowInActiveHunk(row, model.Hunks, state.ActiveHunkID)
		left, right := sideCells(row, styles, active)
		left = padCell(horizontalSlice(left, state.ScrollX, leftWidth), leftWidth)
		right = padCell(horizontalSlice(right, state.ScrollX, rightWidth), rightWidth)
		switch row.Kind {
		case diffpkg.RowAdded:
			right = styles.addedLine.Render(right)
		case diffpkg.RowDeleted:
			left = styles.deletedLine.Render(left)
		case diffpkg.RowChanged:
			left = styles.deletedLine.Render(left)
			right = styles.addedLine.Render(right)
		}
		result = append(result, left+" │ "+right)
	}
	if (state.Density == app.DensityChanges || !model.Document.Capability.FullFile) && !selectedUntracked(state) {
		for _, hunk := range model.Hunks {
			marker := "  "
			if hunk.ID == state.ActiveHunkID {
				marker = "▶ "
			}
			header := fmt.Sprintf("%s@@ -%d,%d +%d,%d @@ [%s]", marker, hunk.BeforeStart, hunk.BeforeLineCount, hunk.AfterStart, hunk.AfterLineCount, hunk.ID)
			result = append(result, styles.hunk.Render(horizontalSlice(header, 0, width)))
			for _, row := range hunk.Rows {
				appendRow(row)
			}
		}
		return result
	}
	for _, row := range model.FullRows {
		appendRow(row)
	}
	return result
}

func sideCells(row diffpkg.Row, styles styles, active bool) (string, string) {
	marker := "  "
	if active {
		marker = "▶ "
	}
	leftMark, rightMark := " ", " "
	switch row.Kind {
	case diffpkg.RowAdded:
		rightMark = "+"
	case diffpkg.RowDeleted:
		leftMark = "-"
	case diffpkg.RowChanged:
		leftMark, rightMark = "-", "+"
	}
	left := sideCell(marker, row.Before, leftMark, styles)
	right := sideCell(marker, row.After, rightMark, styles)
	return left, right
}

func sideCell(marker string, line *diffpkg.Line, changeMark string, styles styles) string {
	if line == nil {
		return strings.Repeat(" ", 8)
	}
	if changeMark == "+" {
		changeMark = styles.addedMark.Render(changeMark)
	} else if changeMark == "-" {
		changeMark = styles.deletedMark.Render(changeMark)
	}
	return fmt.Sprintf("%s%4d %s %s", marker, line.Number, changeMark, renderLine(line, styles))
}

func sideBySideScrollOffset(model diffpkg.Model, density app.Density, rowOffset int) int {
	if rowOffset <= 0 || density != app.DensityChanges {
		return max(0, rowOffset)
	}
	rowsSeen, linesSeen := 0, 0
	for _, hunk := range model.Hunks {
		if rowsSeen == rowOffset {
			return linesSeen
		}
		linesSeen++
		for range hunk.Rows {
			if rowsSeen == rowOffset {
				return linesSeen
			}
			rowsSeen++
			linesSeen++
		}
	}
	return linesSeen
}
