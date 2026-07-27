package app

import (
	"errors"
	"testing"

	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
)

func TestReducerPreservesContextAndActiveHunk(t *testing.T) {
	identity := gitpkg.ChangeIdentity{Path: "selected.go", Scope: gitpkg.ScopeUnstaged}
	model := NewModel()
	model.Snapshot.Changes = []gitpkg.Change{{Path: identity.Path, Scope: identity.Scope}}
	model.Selection, model.HasSelection = identity, true
	model.ScrollY = 50
	diffModel := diffpkg.Build(diffpkg.NewTextDocument(identity.Path, "", []byte("old\n"), true, []byte("new\n"), true), diffpkg.DefaultLimits())
	model.ActiveHunkID = diffModel.Hunks[0].ID
	model.Detail = Detail{Identity: identity, Diff: &diffModel}

	model = Reduce(model, RefreshStarted{Generation: 1, Reason: RefreshManual})
	snapshot := gitpkg.Snapshot{Changes: []gitpkg.Change{
		{Path: "new.go", Scope: gitpkg.ScopeUntracked},
		{Path: identity.Path, Scope: identity.Scope},
	}}
	model = Reduce(model, RefreshCompleted{Generation: 1, Snapshot: snapshot, Detail: Detail{Identity: identity, Diff: &diffModel}})
	if model.Selection != identity || model.ActiveHunkID == "" {
		t.Fatalf("context not preserved: %#v", model)
	}
	if model.ScrollY != len(diffModel.ChangesRows())-1 {
		t.Fatalf("ScrollY = %d, want clamped", model.ScrollY)
	}
}

func TestReducerDiscardsOutOfOrderAndRecoversFromError(t *testing.T) {
	model := NewModel()
	model = Reduce(model, RefreshStarted{Generation: 1, Reason: RefreshInitial})
	model = Reduce(model, RefreshStarted{Generation: 2, Reason: RefreshManual})
	model = Reduce(model, RefreshCompleted{Generation: 1, Snapshot: gitpkg.Snapshot{Changes: []gitpkg.Change{{Path: "old"}}}})
	if len(model.Snapshot.Changes) != 0 || model.Progress.Generation != 2 {
		t.Fatalf("stale completion changed model: %#v", model)
	}
	model = Reduce(model, RefreshCompleted{Generation: 2, Err: errors.New("index locked")})
	if model.Error == nil || model.Progress.Refreshing {
		t.Fatalf("error state = %#v", model)
	}
	model = Reduce(model, RefreshStarted{Generation: 3, Reason: RefreshReconcile})
	model = Reduce(model, RefreshCompleted{Generation: 3, Snapshot: gitpkg.Snapshot{}})
	if model.Error != nil || model.Progress.Refreshing {
		t.Fatalf("model did not recover: %#v", model)
	}
}

func TestReducerLocalizesEntryError(t *testing.T) {
	identity := gitpkg.ChangeIdentity{Path: "gone", Scope: gitpkg.ScopeUnstaged}
	model := NewModel()
	model = Reduce(model, RefreshStarted{Generation: 1, Reason: RefreshInitial})
	entryError := &AppError{Summary: "content load failed", Detail: "file disappeared"}
	model = Reduce(model, RefreshCompleted{
		Generation: 1,
		Snapshot:   gitpkg.Snapshot{Changes: []gitpkg.Change{{Path: identity.Path, Scope: identity.Scope}}},
		Detail:     Detail{Identity: identity, Error: entryError},
	})
	if model.Error != nil || model.Detail.Error != entryError || !model.HasSelection {
		t.Fatalf("entry error was not localized: %#v", model)
	}
}

func TestReducerFollowsPathAcrossScopeEvenFromFilteredView(t *testing.T) {
	oldIdentity := gitpkg.ChangeIdentity{Path: "move.txt", Scope: gitpkg.ScopeUntracked}
	model := NewModel()
	model.Filter = FilterUntracked
	model.Snapshot.Changes = []gitpkg.Change{{Path: oldIdentity.Path, Scope: oldIdentity.Scope}}
	model.Selection, model.HasSelection = oldIdentity, true
	model = Reduce(model, RefreshStarted{Generation: 1, Reason: RefreshWatch})
	newIdentity := gitpkg.ChangeIdentity{Path: oldIdentity.Path, Scope: gitpkg.ScopeStaged}
	model = Reduce(model, RefreshCompleted{
		Generation: 1,
		Snapshot:   gitpkg.Snapshot{Changes: []gitpkg.Change{{Path: newIdentity.Path, Scope: newIdentity.Scope}}},
		Detail:     Detail{Identity: newIdentity},
	})
	if model.Selection != newIdentity || model.Filter != FilterStaged {
		t.Fatalf("scope migration = selection %#v filter %v", model.Selection, model.Filter)
	}
}
