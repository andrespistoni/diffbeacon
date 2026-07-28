package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrespistoni/diffbeacon/internal/app"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestErrorDetailKeepsListAndSanitizesDiagnostic(t *testing.T) {
	change := gitpkg.Change{Path: "kept.go", Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified}
	model := New("repo", nil, nil)
	model.styles = newStyles(false)
	model.state.Snapshot.Changes = []gitpkg.Change{change}
	model.state.Selection, model.state.HasSelection = change.Identity(), true
	model.state.Detail = app.Detail{Identity: change.Identity(), Error: &app.AppError{Summary: "content load failed", Detail: "git failed\n\x1b[31munsafe"}}
	model.width, model.height = 100, 20

	brief := model.View()
	if !containsAll(brief, "kept.go", "content load failed", "Press e") {
		t.Fatalf("brief error view = %q", brief)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	detailed := updated.(Model)
	view := detailed.View()
	assertGolden(t, "testdata/special/error-detail.golden", view)
	if strings.Contains(view, "\x1b[31munsafe") || !strings.Contains(view, `\x1b[31munsafe`) {
		t.Fatalf("error detail was not sanitized: %q", view)
	}
	closed, _ := detailed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if closed.(Model).errorDetails {
		t.Fatal("Esc did not close error detail")
	}
}

func TestErrorRefreshIsNonFatalAndCanRecover(t *testing.T) {
	model := New("repo", nil, nil)
	model.state = app.Reduce(model.state, app.RefreshStarted{Generation: 1, Reason: app.RefreshManual})
	updated, _ := model.Update(app.RefreshCompleted{Generation: 1, Err: &gitpkg.CommandError{Stderr: "index.lock\x1b[31m"}})
	failed := updated.(Model)
	if failed.state.Error == nil || !strings.Contains(failed.View(), "refresh failed") {
		t.Fatalf("failed model = %#v", failed.state)
	}
	failed.state = app.Reduce(failed.state, app.RefreshStarted{Generation: 2, Reason: app.RefreshManual})
	recovered, _ := failed.Update(app.RefreshCompleted{Generation: 2, Snapshot: gitpkg.Snapshot{}})
	if recovered.(Model).state.Error != nil {
		t.Fatalf("error survived successful refresh: %#v", recovered.(Model).state.Error)
	}
}
