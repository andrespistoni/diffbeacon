package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

type fakeBackend struct {
	events chan fsnotify.Event
	errors chan error
	mu     sync.Mutex
	added  []string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{events: make(chan fsnotify.Event, 16), errors: make(chan error, 16)}
}

func (b *fakeBackend) Add(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.added = append(b.added, path)
	return nil
}
func (b *fakeBackend) Close() error                  { return nil }
func (b *fakeBackend) Events() <-chan fsnotify.Event { return b.events }
func (b *fakeBackend) Errors() <-chan error          { return b.errors }

func TestWatcherEmitsEventsReconcilesAndRecovers(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	fake := newFakeBackend()
	watcher.clock = clock
	watcher.newBackend = func() (backend, error) { return fake, nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := watcher.Run(ctx)
	waitReady(t, watcher.Ready())

	fake.events <- fsnotify.Event{Name: filepath.Join(root, "file"), Op: fsnotify.Write}
	waitForTimerCount(t, clock, 3)
	clock.Advance(DefaultDebounce)
	assertSignal(t, signals, ReasonEvent, false)
	assertNoSignal(t, signals)

	clock.Advance(DefaultReconcile - DefaultDebounce)
	assertSignal(t, signals, ReasonReconcile, false)

	fake.errors <- errors.New("temporary watcher failure")
	assertSignal(t, signals, ReasonRecovery, true)
	fake.events <- fsnotify.Event{Name: filepath.Join(gitDir, "index"), Op: fsnotify.Write}
	waitForTimerCount(t, clock, 5)
	clock.Advance(DefaultDebounce)
	assertSignal(t, signals, ReasonEvent, false)
}

func TestWatcherIgnoresMetadataOnlyEvents(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	fake := newFakeBackend()
	watcher.clock = clock
	watcher.newBackend = func() (backend, error) { return fake, nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := watcher.Run(ctx)
	waitReady(t, watcher.Ready())

	fake.events <- fsnotify.Event{Name: filepath.Join(root, "file"), Op: fsnotify.Chmod}
	assertNoSignal(t, signals)
}

func TestWatcherDoesNotFollowDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	external := t.TempDir()
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	if err := watcher.syncWatches(backend); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	for _, path := range backend.added {
		if path == external {
			t.Fatalf("external symlink target was watched: %s", path)
		}
	}
}

func TestWatcherExhaustionIsBoundedAndReconciliationRemainsActive(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	for _, path := range []string{gitDir, filepath.Join(root, "one"), filepath.Join(root, "two"), filepath.Join(root, "three")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := DefaultConfig()
	config.MaxWatches = 2
	watcher, err := New(root, gitDir, config)
	if err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	fake := newFakeBackend()
	watcher.clock = clock
	watcher.newBackend = func() (backend, error) { return fake, nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := watcher.Run(ctx)
	signal := <-signals
	if signal.Reason != ReasonRecovery || !errors.Is(signal.Err, ErrWatchBudget) {
		t.Fatalf("initial signal = %#v, want explicit budget degradation", signal)
	}
	fake.mu.Lock()
	watchCount := len(fake.added)
	fake.mu.Unlock()
	if watchCount > config.MaxWatches {
		t.Fatalf("watch count = %d, limit %d", watchCount, config.MaxWatches)
	}
	waitForTimerCount(t, clock, 1)
	clock.Advance(DefaultReconcile)
	signal = <-signals
	if signal.Reason != ReasonReconcile || !errors.Is(signal.Err, ErrWatchBudget) {
		t.Fatalf("reconciliation signal = %#v, want active degraded reconciliation", signal)
	}
}

func TestWatcherOnlyRecursesRelevantGitReferences(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	for _, path := range []string{filepath.Join(gitDir, "refs", "heads"), filepath.Join(gitDir, "objects", "aa"), filepath.Join(gitDir, "logs", "refs")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	backend := newFakeBackend()
	if err := watcher.syncWatches(backend); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	for _, watched := range backend.added {
		if pathWithin(watched, filepath.Join(gitDir, "objects")) || pathWithin(watched, filepath.Join(gitDir, "logs")) {
			t.Fatalf("irrelevant Git internals were watched: %s", watched)
		}
	}
}

func TestWatcherTraversalTimeBudgetIsDeterministic(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	watcher.now = func() time.Time {
		now = now.Add(DefaultScanTimeout)
		return now
	}
	err = watcher.syncWatches(newFakeBackend())
	if !errors.Is(err, ErrWatchBudget) || !strings.Contains(err.Error(), "traversal exceeded") {
		t.Fatalf("syncWatches() error = %v, want time budget", err)
	}
}

func TestWatcherRecoversAfterTransientBackendCreationFailure(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	fake := newFakeBackend()
	attempts := 0
	watcher.clock = clock
	watcher.newBackend = func() (backend, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary create failure")
		}
		return fake, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := watcher.Run(ctx)
	assertSignal(t, signals, ReasonRecovery, true)
	waitForTimerCount(t, clock, 2) // reconciliation timer plus retry timer
	clock.Advance(DefaultDebounce)
	waitReady(t, watcher.Ready())

	fake.events <- fsnotify.Event{Name: filepath.Join(root, "recovered"), Op: fsnotify.Create}
	waitForTimerCount(t, clock, 4)
	clock.Advance(DefaultDebounce)
	assertSignal(t, signals, ReasonEvent, false)
}

func TestWatcherIncludesWorktreeCommonDirectory(t *testing.T) {
	root := t.TempDir()
	common := filepath.Join(root, ".git")
	gitDir := filepath.Join(common, "worktrees", "topic")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(watcher.gitRoots) != 2 || watcher.gitRoots[1] != common {
		t.Fatalf("git roots = %#v, want worktree Git dir and common dir", watcher.gitRoots)
	}
}

func TestConfigNormalizesToApprovedRanges(t *testing.T) {
	config := (Config{Debounce: time.Millisecond, MaxWait: time.Millisecond, Reconcile: time.Hour}).normalized()
	if config != DefaultConfig() {
		t.Fatalf("normalized config = %#v, want %#v", config, DefaultConfig())
	}
}

func waitReady(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("watcher did not become ready")
	}
}

func assertSignal(t *testing.T, signals <-chan Signal, reason Reason, wantError bool) {
	t.Helper()
	select {
	case signal := <-signals:
		if signal.Reason != reason || (signal.Err != nil) != wantError {
			t.Fatalf("signal = %#v, want reason %q error=%v", signal, reason, wantError)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q signal", reason)
	}
}

func assertNoSignal(t *testing.T, signals <-chan Signal) {
	t.Helper()
	select {
	case signal := <-signals:
		t.Fatalf("unexpected signal: %#v", signal)
	case <-time.After(5 * time.Millisecond):
	}
}
