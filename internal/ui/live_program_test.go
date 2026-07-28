package ui

import (
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrespistoni/diffbeacon/internal/app"
	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/highlight"
	"github.com/andrespistoni/diffbeacon/internal/testrepo"
	watchpkg "github.com/andrespistoni/diffbeacon/internal/watch"
)

func TestDiffBeaconExternalProcess(t *testing.T) {
	handled, err := testrepo.RunExternalProcess()
	if handled && err != nil {
		t.Fatal(err)
	}
}

func TestLiveProgramConnectsWatcherRefreshAndContext(t *testing.T) {
	fixture := testrepo.New(t)
	for _, path := range []string{"a-before.txt", "m-review.txt", "z-after.txt"} {
		fixture.Write(path, "base\nline two\nline three\n")
	}
	fixture.CommitAll("base")
	for _, path := range []string{"a-before.txt", "m-review.txt", "z-after.txt"} {
		fixture.ExternalWrite(path, "base\nchanged two\nline three\n")
	}

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
	coordinator := app.NewCoordinator(app.GitLoader{
		Runner: runner, Repository: repository,
		DiffLimits: diffpkg.DefaultLimits(), HighlightLimits: highlight.DefaultLimits(),
	})
	defer func() {
		cancel()
		coordinator.Close()
	}()

	updates := make(chan Model, 256)
	observed := liveObservedModel{Model: New(repository.Root, coordinator, watcher.Run(ctx)), updates: updates}
	program := tea.NewProgram(observed, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	finished := make(chan tea.Model, 1)
	errors := make(chan error, 1)
	go func() {
		model, err := program.Run()
		if err != nil {
			errors <- err
			return
		}
		finished <- model
	}()
	defer program.Kill()

	current := waitForLiveModel(t, updates, func(model Model) bool {
		return !model.state.Progress.Refreshing && len(model.state.Snapshot.Changes) == 3 && model.state.Detail.Diff != nil
	})
	if current.state.Selection.Path != "a-before.txt" {
		t.Fatalf("initial selection = %#v", current.state.Selection)
	}
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return model.state.Selection == (gitpkg.ChangeIdentity{Path: "m-review.txt", Scope: gitpkg.ScopeUnstaged}) &&
			model.state.Detail.Identity == model.state.Selection && model.state.Detail.Diff != nil
	})

	fixture.ExternalWrite("b-new.txt", "created externally\n")
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return hasLiveChange(model.state, "b-new.txt", gitpkg.ScopeUntracked) && !model.state.Progress.Refreshing
	})
	if current.state.Selection.Path != "m-review.txt" || current.state.Selection.Scope != gitpkg.ScopeUnstaged {
		t.Fatalf("new entry displaced active review: %#v", current.state.Selection)
	}

	fixture.ExternalGit("add", "--", "m-review.txt")
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return model.state.Selection == (gitpkg.ChangeIdentity{Path: "m-review.txt", Scope: gitpkg.ScopeStaged}) &&
			model.state.Detail.Identity == model.state.Selection && !model.state.Progress.Refreshing
	})

	fixture.ExternalWrite("m-review.txt", "base\nworking after external stage\nline three\n")
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return hasLiveChange(model.state, "m-review.txt", gitpkg.ScopeStaged) &&
			hasLiveChange(model.state, "m-review.txt", gitpkg.ScopeUnstaged) && !model.state.Progress.Refreshing
	})
	if current.state.Selection.Scope != gitpkg.ScopeStaged {
		t.Fatalf("double-scope refresh moved selection: %#v", current.state.Selection)
	}

	fixture.ExternalGit("commit", "--quiet", "-m", "external baseline")
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return model.state.Selection == (gitpkg.ChangeIdentity{Path: "m-review.txt", Scope: gitpkg.ScopeUnstaged}) &&
			model.state.Detail.Identity == model.state.Selection && model.state.Detail.Diff != nil &&
			model.state.Detail.Diff.Document.After == "base\nworking after external stage\nline three\n"
	})
	if current.state.Detail.Diff.Document.Before != "base\nchanged two\nline three\n" {
		t.Fatalf("HEAD/index baseline was not recalculated: %q", current.state.Detail.Diff.Document.Before)
	}

	fixture.ExternalWrite("m-review.txt", "base\nchanged two\nline three\n")
	disappearanceStarted := time.Now()
	current = waitForLiveModel(t, updates, func(model Model) bool {
		return model.state.Selection == (gitpkg.ChangeIdentity{Path: "z-after.txt", Scope: gitpkg.ScopeUnstaged}) &&
			model.state.Detail.Identity == model.state.Selection && model.state.Detail.Diff != nil && !model.state.Progress.Refreshing
	})
	if elapsed := time.Since(disappearanceStarted); elapsed >= 1500*time.Millisecond {
		t.Fatalf("replacement detail waited for periodic reconciliation: %v", elapsed)
	}
	if hasLiveChange(current.state, "m-review.txt", gitpkg.ScopeUnstaged) {
		t.Fatal("clean selected entry remained visible")
	}

	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	waitForLiveModel(t, updates, func(model Model) bool {
		return model.state.Progress.Reason == app.RefreshManual
	})
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	select {
	case final := <-finished:
		if _, ok := final.(liveObservedModel); !ok {
			t.Fatalf("final model type = %T", final)
		}
	case err := <-errors:
		t.Fatalf("program failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit")
	}
}

type liveObservedModel struct {
	Model
	updates chan<- Model
}

func (m liveObservedModel) Init() tea.Cmd { return m.Model.Init() }

func (m liveObservedModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	updated, command := m.Model.Update(message)
	m.Model = updated.(Model)
	select {
	case m.updates <- m.Model:
	default:
	}
	return m, command
}

func (m liveObservedModel) View() string { return m.Model.View() }

func waitForLiveModel(t *testing.T, updates <-chan Model, predicate func(Model) bool) Model {
	t.Helper()
	timer := time.NewTimer(6 * time.Second)
	defer timer.Stop()
	var last Model
	for {
		select {
		case last = <-updates:
			if predicate(last) {
				return last
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for live model; selection=%#v changes=%#v progress=%#v detail=%#v", last.state.Selection, last.state.Snapshot.Changes, last.state.Progress, last.state.Detail.Identity)
			return Model{}
		}
	}
}

func hasLiveChange(model app.Model, path string, scope gitpkg.Scope) bool {
	for _, change := range model.Snapshot.Changes {
		if change.Path == path && change.Scope == scope {
			return true
		}
	}
	return false
}
