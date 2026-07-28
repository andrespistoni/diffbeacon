package ui

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrespistoni/diffbeacon/internal/app"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

type blockingCoordinator struct {
	started chan app.RefreshRequest
	release chan struct{}
	results chan app.RefreshCompleted
	once    sync.Once
}

func newBlockingCoordinator() *blockingCoordinator {
	return &blockingCoordinator{
		started: make(chan app.RefreshRequest, 1), release: make(chan struct{}),
		results: make(chan app.RefreshCompleted),
	}
}

func (c *blockingCoordinator) Begin(request app.RefreshRequest) app.RefreshStarted {
	c.started <- request
	<-c.release
	return app.RefreshStarted{Generation: 1, Reason: request.Reason}
}

func (c *blockingCoordinator) Results() <-chan app.RefreshCompleted { return c.results }

func (c *blockingCoordinator) unblock() { c.once.Do(func() { close(c.release) }) }

func TestUpdateRemainsResponsiveDuringSlowRefresh(t *testing.T) {
	coordinator := newBlockingCoordinator()
	defer coordinator.unblock()
	model := New("repo", coordinator, nil)

	startedAt := time.Now()
	updated, refresh := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if time.Since(startedAt) > 50*time.Millisecond {
		t.Fatal("Update blocked before returning the refresh command")
	}
	if refresh == nil {
		t.Fatal("manual refresh returned no command")
	}
	select {
	case <-coordinator.started:
		t.Fatal("Update executed refresh work synchronously")
	default:
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- refresh() }()
	select {
	case request := <-coordinator.started:
		if request.Reason != app.RefreshManual {
			t.Fatalf("refresh reason = %q", request.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh command did not begin")
	}

	helpUpdated, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !helpUpdated.(Model).help.ShowAll {
		t.Fatal("help input was not processed while refresh was blocked")
	}
	_, quit := helpUpdated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if quit == nil {
		t.Fatal("quit input was not processed while refresh was blocked")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatalf("quit command returned %T", quit())
	}
	coordinator.unblock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh command did not finish after release")
	}
}

func TestWindowResizeAndFocusHaveTextualSignals(t *testing.T) {
	model := New("/tmp/repo", nil, nil)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	resized := updated.(Model)
	view := resized.View()
	if resized.width != 80 || resized.height != 20 {
		t.Fatalf("size = %dx%d", resized.width, resized.height)
	}
	if !containsAll(view, "▶ Files / Changes", "All", "Inline", "Changes") {
		t.Fatalf("view lacks textual state signals: %q", view)
	}
}

func TestRefreshKeepsExistingSelectionWhenAnotherEntryAppears(t *testing.T) {
	selected := gitpkg.Change{Path: "kept.go", Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified}
	model := New("repo", nil, nil)
	model.state = app.NewModel()
	model.state.Snapshot.Changes = []gitpkg.Change{selected}
	model.state.Selection, model.state.HasSelection = selected.Identity(), true
	model.state = app.Reduce(model.state, app.RefreshStarted{Generation: 2, Reason: app.RefreshWatch})
	updated, _ := model.Update(app.RefreshCompleted{
		Generation: 2,
		Snapshot: appSnapshot([]gitpkg.Change{
			{Path: "appeared.go", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusAdded},
			selected,
		}),
	})
	got := updated.(Model).state.Selection
	if got != selected.Identity() {
		t.Fatalf("selection moved to %#v, want %#v", got, selected.Identity())
	}
}

func appSnapshot(changes []gitpkg.Change) gitpkg.Snapshot {
	return gitpkg.Snapshot{Changes: changes}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
