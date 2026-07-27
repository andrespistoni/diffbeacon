package app

import (
	"context"
	"sync"
	"testing"
	"time"

	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/highlight"
	"diffbeacon/internal/testrepo"
	watchpkg "diffbeacon/internal/watch"
)

type controlledLoader struct {
	started chan loadCall
}

type loadCall struct {
	ctx     context.Context
	request RefreshRequest
	result  chan loadResult
}

type loadResult struct {
	payload RefreshPayload
	err     error
}

func (l *controlledLoader) Load(ctx context.Context, request RefreshRequest) (RefreshPayload, error) {
	call := loadCall{ctx: ctx, request: request, result: make(chan loadResult, 1)}
	l.started <- call
	result := <-call.result
	return result.payload, result.err
}

func TestCoordinatorDoesNotBlockAndCancelsReplacedWork(t *testing.T) {
	loader := &controlledLoader{started: make(chan loadCall, 2)}
	coordinator := NewCoordinator(loader)
	defer coordinator.Close()

	returned := make(chan RefreshStarted, 1)
	go func() { returned <- coordinator.Begin(RefreshRequest{Reason: RefreshInitial}) }()
	select {
	case started := <-returned:
		if started.Generation != 1 {
			t.Fatalf("generation = %d", started.Generation)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Begin blocked on loader")
	}
	first := <-loader.started
	coordinator.Begin(RefreshRequest{Reason: RefreshManual})
	second := <-loader.started
	select {
	case <-first.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("replaced refresh context was not canceled")
	}
	first.result <- loadResult{err: context.Canceled}
	second.result <- loadResult{}
}

func TestOutOfOrderResultsNeverReplaceNewGeneration(t *testing.T) {
	loader := &controlledLoader{started: make(chan loadCall, 2)}
	coordinator := NewCoordinator(loader)
	defer coordinator.Close()

	model := NewModel()
	model = Reduce(model, coordinator.Begin(RefreshRequest{Reason: RefreshInitial}))
	first := <-loader.started
	model = Reduce(model, coordinator.Begin(RefreshRequest{Reason: RefreshManual}))
	second := <-loader.started

	newSnapshot := gitpkg.Snapshot{Changes: []gitpkg.Change{{Path: "new", Scope: gitpkg.ScopeUntracked}}}
	second.result <- loadResult{payload: RefreshPayload{Snapshot: newSnapshot}}
	model = Reduce(model, receiveResult(t, coordinator.Results()))
	oldSnapshot := gitpkg.Snapshot{Changes: []gitpkg.Change{{Path: "old", Scope: gitpkg.ScopeUntracked}}}
	first.result <- loadResult{payload: RefreshPayload{Snapshot: oldSnapshot}}
	model = Reduce(model, receiveResult(t, coordinator.Results()))
	if len(model.Snapshot.Changes) != 1 || model.Snapshot.Changes[0].Path != "new" {
		t.Fatalf("stale result replaced new snapshot: %#v", model.Snapshot)
	}
}

func TestCoordinatorConcurrentRequestsAreRaceSafe(t *testing.T) {
	loader := immediateLoader{}
	coordinator := NewCoordinator(loader)
	defer coordinator.Close()
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			coordinator.Begin(RefreshRequest{Reason: RefreshWatch})
		}()
	}
	wg.Wait()
	for range 20 {
		_ = receiveResult(t, coordinator.Results())
	}
}

func TestCoordinatorConsumesWatchAndReconciliationSignals(t *testing.T) {
	coordinator := NewCoordinator(immediateLoader{})
	defer coordinator.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan watchpkg.Signal, 2)
	started := coordinator.ConsumeWatch(ctx, signals, func() RefreshRequest { return RefreshRequest{} })
	signals <- watchpkg.Signal{Reason: watchpkg.ReasonEvent}
	signals <- watchpkg.Signal{Reason: watchpkg.ReasonReconcile}
	if message := <-started; message.Reason != RefreshWatch {
		t.Fatalf("event reason = %q", message.Reason)
	}
	if message := <-started; message.Reason != RefreshReconcile {
		t.Fatalf("reconcile reason = %q", message.Reason)
	}
	for range 2 {
		_ = receiveResult(t, coordinator.Results())
	}
}

func TestCoordinatorRunsRealGitContentDiffAndHighlightPipeline(t *testing.T) {
	repositoryFixture := testrepo.New(t)
	repositoryFixture.Write("sample.go", "package sample\n\nconst Value = 1\n")
	repositoryFixture.CommitAll("base")
	repositoryFixture.Write("sample.go", "package sample\n\nconst Value = 2\n")
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, repositoryFixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	coordinator := NewCoordinator(GitLoader{
		Runner: runner, Repository: repository,
		DiffLimits: diffpkg.DefaultLimits(), HighlightLimits: highlight.DefaultLimits(),
	})
	defer coordinator.Close()
	started := coordinator.Begin(RefreshRequest{Reason: RefreshInitial})
	result := receiveResult(t, coordinator.Results())
	if result.Generation != started.Generation || result.Err != nil {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Snapshot.Changes) != 1 || result.Detail.Diff == nil || len(result.Detail.Diff.Hunks) != 1 {
		t.Fatalf("pipeline payload = %#v", result)
	}
	if !result.Detail.Highlight.Applied {
		t.Fatalf("highlight result = %#v", result.Detail.Highlight)
	}
}

type immediateLoader struct{}

func (immediateLoader) Load(context.Context, RefreshRequest) (RefreshPayload, error) {
	return RefreshPayload{}, nil
}

func receiveResult(t *testing.T, results <-chan RefreshCompleted) RefreshCompleted {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for refresh result")
		return RefreshCompleted{}
	}
}
