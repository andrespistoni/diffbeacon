package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/highlight"
)

func TestInlineChangesOnlyGolden(t *testing.T) {
	state := trackedDiffState(t)
	assertGolden(t, "testdata/diff/changes.golden", strings.Join(renderInline(state, newStyles(false)), "\n"))
}

func TestInlineStagedUsesItsOwnBaselineGolden(t *testing.T) {
	state := stagedDiffState(t)
	assertGolden(t, "testdata/diff/staged.golden", strings.Join(renderInline(state, newStyles(false)), "\n"))
	if got, unstaged := strings.Join(renderInline(state, newStyles(false)), "\n"), strings.Join(renderInline(trackedDiffState(t), newStyles(false)), "\n"); got == unstaged {
		t.Fatal("staged and unstaged baselines rendered identically")
	}
}

func TestFullFileGolden(t *testing.T) {
	state := trackedDiffState(t)
	state.Density = app.DensityFullFile
	state.ActiveHunkID = "hunk-2"
	assertGolden(t, "testdata/diff/full.golden", strings.Join(renderInline(state, newStyles(false)), "\n"))
}

func TestUntrackedAlwaysRendersFullFile(t *testing.T) {
	document := diffpkg.NewTextDocument("new.txt", "", nil, false, []byte("first\nsecond\n"), true)
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	state := app.NewModel()
	state.Density = app.DensityChanges
	state.Selection = gitpkg.ChangeIdentity{Path: "new.txt", Scope: gitpkg.ScopeUntracked}
	state.HasSelection = true
	state.Detail = app.Detail{Identity: state.Selection, Diff: &model}
	gotChanges := strings.Join(renderInline(state, newStyles(false)), "\n")
	state.Density = app.DensityFullFile
	gotFull := strings.Join(renderInline(state, newStyles(false)), "\n")
	if gotChanges != gotFull || strings.Contains(gotChanges, "@@") {
		t.Fatalf("untracked render differs by density:\nchanges=%q\nfull=%q", gotChanges, gotFull)
	}
	assertGolden(t, "testdata/diff/untracked.golden", gotChanges)
}

func TestHighlightingAndHostileANSIStaySemanticallySafe(t *testing.T) {
	document := diffpkg.NewTextDocument("sample.go", "", nil, false, []byte("package sample\nvar value = \"\x1b[31mhostile\"\n"), true)
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	result := highlight.Apply(context.Background(), "sample.go", &model, highlight.DefaultLimits())
	if !result.Applied {
		t.Fatalf("highlight fallback = %q", result.FallbackReason)
	}
	state := app.NewModel()
	state.Density = app.DensityFullFile
	state.Selection = gitpkg.ChangeIdentity{Path: "sample.go", Scope: gitpkg.ScopeUntracked}
	state.HasSelection = true
	state.Detail = app.Detail{Identity: state.Selection, Diff: &model, Highlight: result}
	rendered := strings.Join(renderInline(state, newStyles(true)), "\n")
	styledLines := 0
	for _, operation := range model.Operations {
		if operation.After != nil && len(operation.After.Spans) > 0 {
			styledLines++
		}
	}
	if styledLines == 0 {
		t.Fatal("known lexer produced no syntax spans")
	}
	if !strings.Contains(rendered, `\x1b[31mhostile`) {
		t.Fatalf("hostile ANSI was not escaped as visible data: %q", rendered)
	}
	if strings.Contains(rendered, "\x1b[31mhostile") {
		t.Fatalf("hostile ANSI remained executable: %q", rendered)
	}
}

func TestDiffStylesUseBackgroundWithoutOverridingSyntax(t *testing.T) {
	styles := newStyles(true)
	for name, style := range map[string]lipgloss.Style{
		"added":   styles.addedLine,
		"deleted": styles.deletedLine,
	} {
		if _, absent := style.GetBackground().(lipgloss.NoColor); absent {
			t.Errorf("%s line style has no background", name)
		}
		if _, overrides := style.GetForeground().(lipgloss.NoColor); !overrides {
			t.Errorf("%s line style overrides syntax foreground", name)
		}
	}

	stringStyle := tokenStyle("LiteralString", styles)
	if _, absent := stringStyle.GetForeground().(lipgloss.NoColor); absent {
		t.Fatal("string syntax style has no foreground")
	}
	if _, overrides := stringStyle.GetBackground().(lipgloss.NoColor); !overrides {
		t.Fatal("string syntax style overrides diff background")
	}
}

func TestUnknownLexerFallsBackToPlainText(t *testing.T) {
	document := diffpkg.NewTextDocument("sample.unknown-extension", "", nil, false, []byte("plain text\n"), true)
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	result := highlight.Apply(context.Background(), document.Path, &model, highlight.DefaultLimits())
	if result.Applied || result.FallbackReason == "" {
		t.Fatalf("highlight result = %#v", result)
	}
	state := app.NewModel()
	state.Density = app.DensityFullFile
	state.Selection = gitpkg.ChangeIdentity{Path: document.Path, Scope: gitpkg.ScopeUntracked}
	state.HasSelection = true
	state.Detail = app.Detail{Identity: state.Selection, Diff: &model, Highlight: result}
	if rendered := strings.Join(renderInline(state, newStyles(false)), "\n"); !strings.Contains(rendered, "+ plain text") {
		t.Fatalf("plain fallback render = %q", rendered)
	}
}

func TestChangesRemainAvailableWhenFullFileExceedsBudget(t *testing.T) {
	document := diffpkg.Document{
		Path:          "large.txt",
		BeforePresent: true,
		AfterPresent:  true,
		Kind:          diffpkg.ContentText,
		Capability: diffpkg.Capability{
			Hunks:          true,
			FullFileReason: "content exceeds the full-file limit",
		},
		Patch: "@@ -100,2 +100,2 @@\n context\n-old\n+new\n",
	}
	diffModel := diffpkg.Build(document, diffpkg.DefaultLimits())
	state := app.NewModel()
	state.Density = app.DensityFullFile
	state.Selection = gitpkg.ChangeIdentity{Path: document.Path, Scope: gitpkg.ScopeUnstaged}
	state.HasSelection = true
	state.Snapshot.Changes = []gitpkg.Change{{Path: document.Path, Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified}}
	state.Detail = app.Detail{Identity: state.Selection, Diff: &diffModel}

	inline := strings.Join(renderInline(state, newStyles(false)), "\n")
	sideBySide := strings.Join(renderSideBySide(state, newStyles(false), 80), "\n")
	if !strings.Contains(inline, "@@ -100,2 +100,2 @@") || !strings.Contains(inline, "old") || !strings.Contains(inline, "new") {
		t.Fatalf("changes-only inline render = %q", inline)
	}
	if !strings.Contains(sideBySide, "old") || !strings.Contains(sideBySide, "new") {
		t.Fatalf("changes-only side-by-side render = %q", sideBySide)
	}
	model := New("repo", nil, nil)
	model.state = state
	model.styles = newStyles(false)
	if rendered := model.renderContent(); !strings.Contains(rendered, "Full-file view unavailable") || model.effectiveDensity() != app.DensityChanges {
		t.Fatalf("limited full-file presentation = %q, density %v", rendered, model.effectiveDensity())
	}
	model.state = app.Reduce(model.state, app.SetScroll{Vertical: 1})
	if model.state.ScrollY != 1 {
		t.Fatalf("changes-only scroll was clamped as unavailable full-file content: %d", model.state.ScrollY)
	}
}

func TestHunkNavigationMarksAndMovesActiveHunk(t *testing.T) {
	model := New("repo", nil, nil)
	model.state = trackedDiffState(t)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	first := updated.(Model)
	if first.state.ActiveHunkID != "hunk-1" {
		t.Fatalf("first active hunk = %q", first.state.ActiveHunkID)
	}
	updated, _ = first.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	second := updated.(Model)
	if second.state.ActiveHunkID != "hunk-2" || second.state.ScrollY <= first.state.ScrollY {
		t.Fatalf("second hunk state = id %q scroll %d, first scroll %d", second.state.ActiveHunkID, second.state.ScrollY, first.state.ScrollY)
	}
	lines := strings.Join(renderInline(second.state, newStyles(false)), "\n")
	if !strings.Contains(lines, "▶ @@") || !strings.Contains(lines, "[hunk-2]") {
		t.Fatalf("active hunk is not visible: %q", lines)
	}
	second.styles = newStyles(false)
	visible := second.renderContent()
	if !strings.HasPrefix(visible, "▶ @@") || !strings.Contains(strings.Split(visible, "\n")[0], "[hunk-2]") {
		t.Fatalf("hunk navigation did not scroll to active header: %q", visible)
	}
}

func trackedDiffState(t *testing.T) app.Model {
	t.Helper()
	return diffState(t, gitpkg.ScopeUnstaged, "1", "2")
}

func stagedDiffState(t *testing.T) app.Model {
	t.Helper()
	return diffState(t, gitpkg.ScopeStaged, "0", "1")
}

func diffState(t *testing.T, scope gitpkg.Scope, beforeValue, afterValue string) app.Model {
	t.Helper()
	before := "package sample\n\nconst first = " + beforeValue + "\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nconst second = " + beforeValue + "\nline13\n"
	after := "package sample\n\nconst first = " + afterValue + "\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nconst second = " + afterValue + "\nline13\n"
	document := diffpkg.NewTextDocument("sample.go", "", []byte(before), true, []byte(after), true)
	document.Patch = "@@ -1,6 +1,6 @@\n package sample\n \n-const first = " + beforeValue + "\n+const first = " + afterValue + "\n line4\n line5\n line6\n@@ -9,5 +9,5 @@\n line9\n line10\n line11\n-const second = " + beforeValue + "\n+const second = " + afterValue + "\n line13\n"
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	if len(model.Hunks) != 2 {
		t.Fatalf("fixture hunks = %d, want 2", len(model.Hunks))
	}
	_ = highlight.Apply(context.Background(), "sample.go", &model, highlight.DefaultLimits())
	state := app.NewModel()
	state.Selection = gitpkg.ChangeIdentity{Path: "sample.go", Scope: scope}
	state.HasSelection = true
	state.Detail = app.Detail{Identity: state.Selection, Diff: &model}
	return state
}
