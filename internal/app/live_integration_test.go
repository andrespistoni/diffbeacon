package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/highlight"
	"diffbeacon/internal/testrepo"
	watchpkg "diffbeacon/internal/watch"
)

func TestDiffBeaconExternalProcess(t *testing.T) {
	handled, err := testrepo.RunExternalProcess()
	if handled && err != nil {
		t.Fatal(err)
	}
}

func TestLiveExternalChangesPreserveContextAndMigrateScope(t *testing.T) {
	repositoryFixture := testrepo.New(t)
	for _, path := range []string{"a-before.txt", "m-review.txt", "z-after.txt"} {
		repositoryFixture.Write(path, "base\nline two\nline three\n")
	}
	repositoryFixture.CommitAll("base")
	for _, path := range []string{"a-before.txt", "m-review.txt", "z-after.txt"} {
		repositoryFixture.ExternalWrite(path, "base\nchanged two\nline three\n")
	}

	harness := newLiveHarness(t, repositoryFixture)
	defer harness.close()
	harness.refresh(RefreshInitial)
	harness.model = Reduce(harness.model, SelectChange{Identity: gitpkg.ChangeIdentity{Path: "m-review.txt", Scope: gitpkg.ScopeUnstaged}})
	harness.refresh(RefreshManual)
	if harness.model.Detail.Diff == nil || len(harness.model.Detail.Diff.Hunks) == 0 {
		t.Fatalf("selected detail was not loaded: %#v", harness.model.Detail)
	}
	hunkID := harness.model.Detail.Diff.Hunks[0].ID
	harness.model = Reduce(harness.model, SetActiveHunk{HunkID: hunkID})
	harness.model = Reduce(harness.model, SetScroll{Vertical: 1, Horizontal: 3})

	repositoryFixture.ExternalWrite("new external.txt", "created by another process\n")
	harness.refreshForEvent()
	assertSelection(t, harness.model, "m-review.txt", gitpkg.ScopeUnstaged)
	if harness.model.ActiveHunkID != hunkID || harness.model.ScrollX != 3 {
		t.Fatalf("context changed after unrelated appearance: hunk=%q scroll=(%d,%d)", harness.model.ActiveHunkID, harness.model.ScrollY, harness.model.ScrollX)
	}

	repositoryFixture.ExternalWrite("m-review.txt", "base\nchanged again\nline three\n")
	harness.refreshForEvent()
	assertSelection(t, harness.model, "m-review.txt", gitpkg.ScopeUnstaged)
	if harness.model.Detail.Diff == nil || harness.model.Detail.Diff.Document.After != "base\nchanged again\nline three\n" {
		t.Fatalf("selected content was not recalculated: %#v", harness.model.Detail.Diff)
	}

	repositoryFixture.ExternalGit("add", "--", "m-review.txt")
	harness.refreshForEvent()
	assertSelection(t, harness.model, "m-review.txt", gitpkg.ScopeStaged)

	repositoryFixture.ExternalGit("reset", "--quiet", "HEAD", "--", "m-review.txt")
	harness.refreshForEvent()
	assertSelection(t, harness.model, "m-review.txt", gitpkg.ScopeUnstaged)

	repositoryFixture.ExternalWrite("m-review.txt", "base\nline two\nline three\n")
	harness.refreshForEvent()
	assertSelection(t, harness.model, "new external.txt", gitpkg.ScopeUntracked)
	if changePresent(harness.model.Snapshot, "m-review.txt") {
		t.Fatalf("clean entry remained in snapshot: %#v", harness.model.Snapshot.Changes)
	}
}

func TestLiveReconcileRepairsMissedEventAndIndexLockRecovers(t *testing.T) {
	repositoryFixture := testrepo.New(t)
	repositoryFixture.Write("tracked.txt", "base\n")
	repositoryFixture.CommitAll("base")
	harness := newLiveHarness(t, repositoryFixture)
	defer harness.close()
	harness.refresh(RefreshInitial)

	repositoryFixture.ExternalWrite("missed.txt", "not delivered to the coordinator\n")
	if changePresent(harness.model.Snapshot, "missed.txt") {
		t.Fatal("state changed without a refresh")
	}
	harness.refreshForReconcile()
	if !changePresent(harness.model.Snapshot, "missed.txt") {
		t.Fatalf("reconciliation did not recover missed change: %#v", harness.model.Snapshot.Changes)
	}

	repositoryFixture.ExternalWrite("tracked.txt", "changed\n")
	harness.refreshForEvent()
	harness.model = Reduce(harness.model, SelectChange{Identity: gitpkg.ChangeIdentity{Path: "tracked.txt", Scope: gitpkg.ScopeUnstaged}})
	harness.refresh(RefreshManual)
	lockPath := filepath.Join(harness.repository.GitDir, "index.lock")
	if err := os.WriteFile(lockPath, []byte("owned by another process"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness.refresh(RefreshManual)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("DiffBeacon removed another process's lock: %v", err)
	}

	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	harness.refresh(RefreshManual)
	if harness.model.Error != nil || !changePresent(harness.model.Snapshot, "tracked.txt") {
		t.Fatalf("refresh did not recover after lock removal: error=%v changes=%#v", harness.model.Error, harness.model.Snapshot.Changes)
	}
}

func TestLiveOutOfOrderRealSnapshotsKeepNewestGeneration(t *testing.T) {
	repositoryFixture := testrepo.New(t)
	repositoryFixture.Write("old.txt", "old\n")
	oldSnapshot := queryLiveSnapshot(t, repositoryFixture)
	repositoryFixture.ExternalWrite("new.txt", "new\n")
	newSnapshot := queryLiveSnapshot(t, repositoryFixture)

	loader := &controlledLoader{started: make(chan loadCall, 2)}
	coordinator := NewCoordinator(loader)
	defer coordinator.Close()
	model := NewModel()
	model = Reduce(model, coordinator.Begin(RefreshRequest{Reason: RefreshInitial}))
	first := <-loader.started
	model = Reduce(model, coordinator.Begin(RefreshRequest{Reason: RefreshWatch}))
	second := <-loader.started
	second.result <- loadResult{payload: RefreshPayload{Snapshot: newSnapshot}}
	model = Reduce(model, receiveResult(t, coordinator.Results()))
	first.result <- loadResult{payload: RefreshPayload{Snapshot: oldSnapshot}}
	model = Reduce(model, receiveResult(t, coordinator.Results()))
	if !changePresent(model.Snapshot, "new.txt") || len(model.Snapshot.Changes) != len(newSnapshot.Changes) {
		t.Fatalf("older real snapshot replaced newer generation: %#v", model.Snapshot.Changes)
	}
}

type liveHarness struct {
	t           *testing.T
	ctx         context.Context
	cancel      context.CancelFunc
	repository  gitpkg.Repository
	coordinator *Coordinator
	watcher     *watchpkg.Watcher
	signals     <-chan watchpkg.Signal
	model       Model
}

func newLiveHarness(t *testing.T, fixture *testrepo.Repository) *liveHarness {
	t.Helper()
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	watcher, err := watchpkg.New(repository.Root, repository.GitDir, watchpkg.Config{
		Debounce: 100 * time.Millisecond, MaxWait: 300 * time.Millisecond, Reconcile: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	coordinator := NewCoordinator(GitLoader{
		Runner: runner, Repository: repository,
		DiffLimits: diffpkg.DefaultLimits(), HighlightLimits: highlight.DefaultLimits(),
	})
	harness := &liveHarness{
		t: t, ctx: ctx, cancel: cancel, repository: repository, coordinator: coordinator,
		watcher: watcher, signals: watcher.Run(ctx), model: NewModel(),
	}
	select {
	case <-watcher.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not become ready")
	}
	return harness
}

func (h *liveHarness) close() {
	h.cancel()
	h.coordinator.Close()
}

func (h *liveHarness) refresh(reason RefreshReason) {
	h.t.Helper()
	request := RefreshRequest{Reason: reason, Selection: h.model.Selection, HasSelection: h.model.HasSelection}
	h.model = Reduce(h.model, h.coordinator.Begin(request))
	result := receiveResult(h.t, h.coordinator.Results())
	h.model = Reduce(h.model, result)
}

func (h *liveHarness) refreshForEvent() {
	h.t.Helper()
	deadline := time.After(4 * time.Second)
	for {
		select {
		case signal := <-h.signals:
			if signal.Reason == watchpkg.ReasonEvent {
				h.refresh(RefreshWatch)
				return
			}
		case <-deadline:
			h.t.Fatal("timed out waiting for filesystem event")
		}
	}
}

func (h *liveHarness) refreshForReconcile() {
	h.t.Helper()
	deadline := time.After(4 * time.Second)
	for {
		select {
		case signal := <-h.signals:
			if signal.Reason == watchpkg.ReasonReconcile {
				h.refresh(RefreshReconcile)
				return
			}
		case <-deadline:
			h.t.Fatal("timed out waiting for reconciliation")
		}
	}
}

func assertSelection(t *testing.T, model Model, path string, scope gitpkg.Scope) {
	t.Helper()
	want := gitpkg.ChangeIdentity{Path: path, Scope: scope}
	if !model.HasSelection || model.Selection != want {
		t.Fatalf("selection = %#v (%v), want %#v", model.Selection, model.HasSelection, want)
	}
}

func changePresent(snapshot gitpkg.Snapshot, path string) bool {
	for _, change := range snapshot.Changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

func queryLiveSnapshot(t *testing.T, fixture *testrepo.Repository) gitpkg.Snapshot {
	t.Helper()
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
