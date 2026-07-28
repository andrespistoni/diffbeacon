package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrespistoni/diffbeacon/internal/app"
	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestSideBySideAlignmentGolden(t *testing.T) {
	document := diffpkg.NewTextDocument("aligned.txt", "", []byte("alpha\nremove one\nremove two\nomega\n"), true, []byte("alpha\ninsert one\nomega\nadded tail\n"), true)
	document.Patch = "@@ -1,4 +1,4 @@\n alpha\n-remove one\n-remove two\n+insert one\n omega\n+added tail\n"
	diffModel := diffpkg.Build(document, diffpkg.DefaultLimits())
	state := app.NewModel()
	state.Selection, state.HasSelection = gitpkg.ChangeIdentity{Path: document.Path, Scope: gitpkg.ScopeUnstaged}, true
	state.Detail = app.Detail{Identity: state.Selection, Diff: &diffModel}
	state.Layout = app.LayoutSideBySide
	state.ActiveHunkID = "hunk-1"
	got := strings.Join(renderSideBySide(state, newStyles(false), 76), "\n")
	assertGolden(t, "testdata/layout/side-by-side.golden", got)
	if !strings.Contains(got, "remove two") || !strings.Contains(got, "added tail") {
		t.Fatalf("insert/delete rows missing: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line != "" && !strings.Contains(line, "@@") && !strings.Contains(line, " │ ") {
			t.Fatalf("unaligned row: %q", line)
		}
	}
}

func TestSideBySideScrollMovesBothSidesTogether(t *testing.T) {
	state := trackedDiffState(t)
	state.Layout = app.LayoutSideBySide
	state.ScrollX = 12
	lines := renderSideBySide(state, newStyles(false), 76)
	if len(lines) < 2 {
		t.Fatalf("lines = %#v", lines)
	}
	parts := strings.SplitN(lines[1], " │ ", 2)
	if len(parts) != 2 || strings.Contains(parts[0], "package") || strings.Contains(parts[1], "package") {
		t.Fatalf("horizontal scroll was not synchronized: %q", lines[1])
	}
	state.ScrollY = 1
	offset := sideBySideScrollOffset(*state.Detail.Diff, state.Density, state.ScrollY)
	if offset <= 0 {
		t.Fatalf("vertical offset = %d", offset)
	}
}

func TestSmallTerminalUsesCompactInlineFallbackGolden(t *testing.T) {
	state := trackedDiffState(t)
	change := gitpkg.Change{Path: state.Selection.Path, Scope: state.Selection.Scope, Status: gitpkg.StatusModified}
	state.Snapshot.Changes = []gitpkg.Change{change}
	state.Layout = app.LayoutSideBySide
	state.Focus = app.FocusContent
	model := New("repo", nil, nil)
	model.state = state
	model.styles = newStyles(false)
	model.width, model.height = 54, 12
	assertGolden(t, "testdata/layout/small-terminal.golden", model.View())
	if view := model.View(); !strings.Contains(view, "Compact layout") || !strings.Contains(view, "unavailable at this width") {
		t.Fatalf("small terminal did not explain fallback: %q", view)
	} else if rows := strings.Count(view, "\n") + 1; rows > model.height {
		t.Fatalf("small terminal render uses %d rows, height is %d", rows, model.height)
	}
}

func TestDeterministicRenderAfterResize(t *testing.T) {
	model := New("repo", nil, nil)
	model.state = trackedDiffState(t)
	model.state.Layout = app.LayoutSideBySide
	model.styles = newStyles(false)
	for _, size := range [][2]int{{120, 30}, {60, 10}, {120, 30}} {
		updated, _ := model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		model = updated.(Model)
		if first, second := model.View(), model.View(); first != second {
			t.Fatalf("render changed at %dx%d", size[0], size[1])
		}
	}
}
