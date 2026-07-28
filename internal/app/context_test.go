package app

import (
	"testing"

	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestReconcileSelectionPrefersIdentityThenSamePath(t *testing.T) {
	selected := gitpkg.ChangeIdentity{Path: "keep", Scope: gitpkg.ScopeUnstaged}
	old := []gitpkg.Change{{Path: "keep", Scope: gitpkg.ScopeUnstaged}}
	withNewEntry := []gitpkg.Change{{Path: "aaa", Scope: gitpkg.ScopeUntracked}, {Path: "keep", Scope: gitpkg.ScopeUnstaged}}
	got, ok := reconcileSelection(old, withNewEntry, selected, true)
	if !ok || got != selected {
		t.Fatalf("selection = %#v, %v; want %#v", got, ok, selected)
	}

	moved := []gitpkg.Change{{Path: "keep", Scope: gitpkg.ScopeStaged}, {Path: "other", Scope: gitpkg.ScopeUnstaged}}
	got, ok = reconcileSelection(old, moved, selected, true)
	want := gitpkg.ChangeIdentity{Path: "keep", Scope: gitpkg.ScopeStaged}
	if !ok || got != want {
		t.Fatalf("moved selection = %#v, %v; want %#v", got, ok, want)
	}
}

func TestReconcileSelectionUsesNextThenPreviousNeighbor(t *testing.T) {
	old := []gitpkg.Change{
		{Path: "a", Scope: gitpkg.ScopeUnstaged},
		{Path: "b", Scope: gitpkg.ScopeUnstaged},
		{Path: "c", Scope: gitpkg.ScopeUnstaged},
	}
	selected := old[1].Identity()
	got, _ := reconcileSelection(old, []gitpkg.Change{old[0], old[2]}, selected, true)
	if got != old[2].Identity() {
		t.Fatalf("selection = %#v, want next %#v", got, old[2].Identity())
	}
	got, _ = reconcileSelection(old, []gitpkg.Change{old[0]}, selected, true)
	if got != old[0].Identity() {
		t.Fatalf("selection = %#v, want previous %#v", got, old[0].Identity())
	}
}

func TestHunkAndScrollNormalization(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("a.go", "", []byte("one\ntwo\n"), true, []byte("one\nchanged\n"), true), diffpkg.DefaultLimits())
	if got := normalizeHunk(model.Hunks[0].ID, &model); got == "" {
		t.Fatal("existing hunk was not preserved")
	}
	if got := normalizeHunk("missing", &model); got != "" {
		t.Fatalf("missing hunk normalized to %q", got)
	}
	if got := clampScroll(100, len(model.FullRows)); got != len(model.FullRows)-1 {
		t.Fatalf("clampScroll() = %d", got)
	}
}

func TestReconcileHunkUsesRangesRatherThanUnstableOrdinal(t *testing.T) {
	previous := &diffpkg.Model{Hunks: []diffpkg.Hunk{{ID: "hunk-1", BeforeStart: 10, BeforeLineCount: 2, AfterStart: 10, AfterLineCount: 3}}}
	next := &diffpkg.Model{Hunks: []diffpkg.Hunk{
		{ID: "hunk-1", BeforeStart: 1, BeforeLineCount: 1, AfterStart: 1, AfterLineCount: 1},
		{ID: "hunk-2", BeforeStart: 10, BeforeLineCount: 2, AfterStart: 10, AfterLineCount: 3},
	}}
	if got := reconcileHunk("hunk-1", previous, next); got != "hunk-2" {
		t.Fatalf("reconcileHunk() = %q, want hunk-2", got)
	}
}
