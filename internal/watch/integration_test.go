package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherIntegrationObservesWorkingTreeIndexHEADAndNewDirectories(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, Config{Debounce: 100 * time.Millisecond, MaxWait: 300 * time.Millisecond, Reconcile: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := watcher.Run(ctx)
	waitReady(t, watcher.Ready())

	writeAndWait := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		waitForEventSignal(t, signals)
	}
	writeAndWait(filepath.Join(root, "tracked.txt"), "working tree")
	writeAndWait(filepath.Join(gitDir, "index"), "index")
	writeAndWait(filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")

	nested := filepath.Join(root, "new-directory")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	waitForEventSignal(t, signals)
	writeAndWait(filepath.Join(nested, "new.txt"), "new")
}

func TestPeriodicReconciliationRepairsDeliberatelyMissedEvent(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := New(root, gitDir, Config{Debounce: 100 * time.Millisecond, MaxWait: 300 * time.Millisecond, Reconcile: 2 * time.Second})
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

	// No fsnotify event is sent: this represents an overflow or missed event.
	if err := os.WriteFile(filepath.Join(root, "missed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Second)
	assertSignal(t, signals, ReasonReconcile, false)
}

func waitForEventSignal(t *testing.T, signals <-chan Signal) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case signal := <-signals:
			if signal.Reason == ReasonEvent {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for filesystem event")
		}
	}
}
