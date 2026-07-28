package app

import (
	"testing"

	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestVisibleChangesGroupsUntrackedWithWorkingTreeChanges(t *testing.T) {
	model := NewModel()
	model.Snapshot.Changes = []gitpkg.Change{
		{Path: "a", Scope: gitpkg.ScopeStaged},
		{Path: "b", Scope: gitpkg.ScopeUnstaged},
		{Path: "c", Scope: gitpkg.ScopeUntracked},
	}
	model.Filter = FilterChanges
	got := model.VisibleChanges()
	if len(got) != 2 || got[0].Path != "b" || got[1].Path != "c" {
		t.Fatalf("VisibleChanges() = %#v, want unstaged and untracked", got)
	}
	model.Filter = FilterUntracked
	got = model.VisibleChanges()
	if len(got) != 1 || got[0].Path != "c" {
		t.Fatalf("VisibleChanges() = %#v, want only untracked", got)
	}
}

func TestNewModelHasDeterministicAccessibleModes(t *testing.T) {
	model := NewModel()
	if model.Filter != FilterAll || model.Layout != LayoutInline || model.Density != DensityChanges || model.Focus != FocusChanges {
		t.Fatalf("NewModel() = %#v", model)
	}
}
