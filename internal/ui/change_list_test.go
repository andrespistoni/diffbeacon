package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andrespistoni/diffbeacon/internal/app"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestChangeListAllScopesGolden(t *testing.T) {
	state := app.NewModel()
	state.Snapshot.Changes = []gitpkg.Change{
		{Path: "new.go", OldPath: "old.go", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusRenamed},
		{Path: "same.go", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusModified},
		{Path: "same.go", Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified, Conflict: gitpkg.ConflictBothModified},
		{Path: "vendor/lib", Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified, Submodule: gitpkg.SubmoduleState{IsSubmodule: true, CommitChanged: true}},
		{Path: "notes.txt", Scope: gitpkg.ScopeUntracked, Status: gitpkg.StatusUntracked},
	}
	state.Selection = gitpkg.ChangeIdentity{Path: "same.go", Scope: gitpkg.ScopeUnstaged}
	state.HasSelection = true
	assertGolden(t, "testdata/list/all.golden", renderChangeList(state, newStyles(false)))
}

func TestChangeListEscapesTerminalControlsInPaths(t *testing.T) {
	change := gitpkg.Change{Path: "line\nbreak\x1b[31m.go", Scope: gitpkg.ScopeUntracked, Status: gitpkg.StatusUntracked}
	line := formatChange(change, false)
	if strings.ContainsAny(line, "\n\x1b") || !strings.Contains(line, `line\x0abreak\x1b[31m.go`) {
		t.Fatalf("unsafe path render = %q", line)
	}
}

func TestCleanStateGolden(t *testing.T) {
	state := app.NewModel()
	assertGolden(t, "testdata/list/clean.golden", renderChangeList(state, newStyles(false)))
}

func TestScopeFiltersIncludeUntrackedInChanges(t *testing.T) {
	changes := []gitpkg.Change{
		{Path: "staged", Scope: gitpkg.ScopeStaged},
		{Path: "unstaged", Scope: gitpkg.ScopeUnstaged},
		{Path: "untracked", Scope: gitpkg.ScopeUntracked},
	}
	for _, test := range []struct {
		filter app.Filter
		want   []gitpkg.Scope
	}{
		{app.FilterStaged, []gitpkg.Scope{gitpkg.ScopeStaged}},
		{app.FilterChanges, []gitpkg.Scope{gitpkg.ScopeUnstaged, gitpkg.ScopeUntracked}},
		{app.FilterUntracked, []gitpkg.Scope{gitpkg.ScopeUntracked}},
	} {
		state := app.NewModel()
		state.Snapshot.Changes = changes
		state.Filter = test.filter
		visible := state.VisibleChanges()
		if len(visible) != len(test.want) {
			t.Fatalf("filter %v visible = %#v, want %v", test.filter, visible, test.want)
		}
		for index, scope := range test.want {
			if visible[index].Scope != scope {
				t.Fatalf("filter %v visible = %#v, want %v", test.filter, visible, test.want)
			}
		}
	}
}

func TestChangeListDoubleScopeSelectsExactIdentity(t *testing.T) {
	state := app.NewModel()
	state.Snapshot.Changes = []gitpkg.Change{
		{Path: "same.go", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusModified},
		{Path: "same.go", Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified},
	}
	model := New("repo", nil, nil)
	model.styles = newStyles(false)
	model.state = state
	model.state.Selection = gitpkg.ChangeIdentity{Path: "same.go", Scope: gitpkg.ScopeStaged}
	model.state.HasSelection = true
	updated, _ := model.moveSelection(1)
	got := updated.(Model).state.Selection
	want := (gitpkg.ChangeIdentity{Path: "same.go", Scope: gitpkg.ScopeUnstaged})
	if got != want {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestChangeListScrollsToKeepKeyboardSelectionVisible(t *testing.T) {
	model := New("repo", nil, nil)
	model.styles = newStyles(false)
	model.width = 80
	model.height = 12
	for index := range 12 {
		model.state.Snapshot.Changes = append(model.state.Snapshot.Changes, gitpkg.Change{
			Path:   fmt.Sprintf("file-%02d.go", index),
			Scope:  gitpkg.ScopeStaged,
			Status: gitpkg.StatusModified,
		})
	}
	model.state.Selection = model.state.Snapshot.Changes[0].Identity()
	model.state.HasSelection = true

	for range 11 {
		updated, _ := model.moveSelection(1)
		model = updated.(Model)
		if selected := model.state.Selection.Path; !strings.Contains(model.View(), selected) {
			t.Fatalf("selected file %q is outside the visible list", selected)
		}
	}
	if model.listScroll == 0 {
		t.Fatal("list did not scroll after selection moved below the initial viewport")
	}
}
