package watch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	DefaultMaxScanEntries = 250_000
	DefaultMaxWatches     = 50_000
	DefaultScanTimeout    = 250 * time.Millisecond
)

var ErrWatchBudget = errors.New("watch budget exhausted; filesystem events are partial and periodic Git reconciliation remains active")

type Reason string

const (
	ReasonEvent     Reason = "filesystem-event"
	ReasonReconcile Reason = "periodic-reconciliation"
	ReasonRecovery  Reason = "watcher-recovery"
)

type Signal struct {
	Reason Reason
	Err    error
}

type Config struct {
	Debounce       time.Duration
	MaxWait        time.Duration
	Reconcile      time.Duration
	MaxScanEntries int
	MaxWatches     int
	ScanTimeout    time.Duration
}

func DefaultConfig() Config {
	return Config{
		Debounce: DefaultDebounce, MaxWait: DefaultMaxWait, Reconcile: DefaultReconcile,
		MaxScanEntries: DefaultMaxScanEntries, MaxWatches: DefaultMaxWatches, ScanTimeout: DefaultScanTimeout,
	}
}

func (c Config) normalized() Config {
	if c.Debounce < 100*time.Millisecond || c.Debounce > 250*time.Millisecond {
		c.Debounce = DefaultDebounce
	}
	if c.MaxWait < c.Debounce {
		c.MaxWait = DefaultMaxWait
	}
	if c.Reconcile < 2*time.Second || c.Reconcile > 5*time.Second {
		c.Reconcile = DefaultReconcile
	}
	if c.MaxScanEntries <= 0 {
		c.MaxScanEntries = DefaultMaxScanEntries
	}
	if c.MaxWatches <= 0 {
		c.MaxWatches = DefaultMaxWatches
	}
	if c.ScanTimeout <= 0 {
		c.ScanTimeout = DefaultScanTimeout
	}
	return c
}

type backend interface {
	Add(string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type fsnotifyBackend struct{ watcher *fsnotify.Watcher }

func (b *fsnotifyBackend) Add(path string) error         { return b.watcher.Add(path) }
func (b *fsnotifyBackend) Close() error                  { return b.watcher.Close() }
func (b *fsnotifyBackend) Events() <-chan fsnotify.Event { return b.watcher.Events }
func (b *fsnotifyBackend) Errors() <-chan error          { return b.watcher.Errors }

type Watcher struct {
	root, gitDir string
	gitRoots     []string
	config       Config
	clock        Clock
	newBackend   func() (backend, error)
	now          func() time.Time
	watched      map[string]struct{}

	ready     chan struct{}
	readyOnce sync.Once
}

func New(root, gitDir string, config Config) (*Watcher, error) {
	root, err := cleanDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("watch working tree: %w", err)
	}
	gitDir, err = cleanDirectory(gitDir)
	if err != nil {
		return nil, fmt.Errorf("watch Git directory: %w", err)
	}
	gitRoots := []string{gitDir}
	if commonDir, ok := resolveCommonDir(gitDir); ok && commonDir != gitDir {
		gitRoots = append(gitRoots, commonDir)
	}
	return &Watcher{
		root: root, gitDir: gitDir, gitRoots: gitRoots, config: config.normalized(), clock: realClock{}, now: time.Now,
		watched: make(map[string]struct{}), ready: make(chan struct{}),
		newBackend: func() (backend, error) {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return nil, err
			}
			return &fsnotifyBackend{watcher: watcher}, nil
		},
	}, nil
}

func resolveCommonDir(gitDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", false
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(gitDir, value)
	}
	value = filepath.Clean(value)
	info, err := os.Stat(value)
	return value, err == nil && info.IsDir()
}

func cleanDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}
	return filepath.Clean(absolute), nil
}

func (w *Watcher) Ready() <-chan struct{} { return w.ready }

func (w *Watcher) Run(ctx context.Context) <-chan Signal {
	output := make(chan Signal, 16)
	go w.run(ctx, output)
	return output
}

func (w *Watcher) run(ctx context.Context, output chan<- Signal) {
	defer close(output)
	events := make(chan struct{}, 32)
	debounced := make(chan struct{}, 1)
	debouncer := NewDebouncer(w.config.Debounce, w.config.MaxWait)
	debouncer.clock = w.clock
	go debouncer.Run(ctx, events, debounced)

	reconcileTimer := w.clock.NewTimer(w.config.Reconcile)
	defer stopTimer(reconcileTimer)

	var current backend
	for {
		if current == nil {
			created, err := w.newBackend()
			if err != nil {
				sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: err})
				if !waitForRetry(ctx, w.clock, w.config.Debounce) {
					return
				}
				continue
			}
			current = created
			w.watched = make(map[string]struct{})
			if err := w.syncWatches(current); err != nil {
				sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: err})
			}
			w.readyOnce.Do(func() { close(w.ready) })
		}

		select {
		case <-ctx.Done():
			_ = current.Close()
			return
		case event, ok := <-current.Events():
			if !ok {
				_ = current.Close()
				current = nil
				sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: errors.New("filesystem event channel closed")})
				if !waitForRetry(ctx, w.clock, w.config.Debounce) {
					return
				}
				continue
			}
			if event.Has(fsnotify.Create) {
				budget := w.newScanBudget()
				if err := w.addDirectoryTree(current, event.Name, &budget); err != nil && !errors.Is(err, fs.ErrNotExist) {
					sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: err})
				}
			}
			select {
			case events <- struct{}{}:
			default:
			}
		case err, ok := <-current.Errors():
			if !ok {
				_ = current.Close()
				current = nil
				sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: errors.New("filesystem error channel closed")})
				if !waitForRetry(ctx, w.clock, w.config.Debounce) {
					return
				}
				continue
			}
			sendSignal(ctx, output, Signal{Reason: ReasonRecovery, Err: err})
		case <-debounced:
			sendSignal(ctx, output, Signal{Reason: ReasonEvent})
		case <-reconcileTimer.C():
			err := w.syncWatches(current)
			sendSignal(ctx, output, Signal{Reason: ReasonReconcile, Err: err})
			reconcileTimer.Reset(w.config.Reconcile)
		}
	}
}

func (w *Watcher) syncWatches(target backend) error {
	var problems []error
	budget := w.newScanBudget()
	if err := w.addDirectoryTree(target, w.root, &budget); err != nil {
		problems = append(problems, err)
	}
	for _, gitRoot := range w.gitRoots {
		if err := w.addDirectory(target, gitRoot); err != nil && !errors.Is(err, fs.ErrNotExist) {
			problems = append(problems, err)
		}
		if err := w.addDirectoryTree(target, filepath.Join(gitRoot, "refs"), &budget); err != nil && !errors.Is(err, fs.ErrNotExist) {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

type scanBudget struct {
	entries  int
	deadline time.Time
}

func (w *Watcher) newScanBudget() scanBudget {
	return scanBudget{deadline: w.now().Add(w.config.ScanTimeout)}
}

func (w *Watcher) addDirectoryTree(target backend, root string, budget *scanBudget) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil
	}
	stack := []string{root}
	for len(stack) > 0 {
		path := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if err := w.consumeScanBudget(budget); err != nil {
			return err
		}
		if root == w.root && path != root && pathWithin(path, w.gitDir) {
			continue
		}
		if err := w.addDirectory(target, path); err != nil {
			return err
		}
		directory, err := os.Open(path)
		if err != nil {
			return err
		}
		for {
			entries, readErr := directory.ReadDir(256)
			for _, entry := range entries {
				if err := w.consumeScanBudget(budget); err != nil {
					_ = directory.Close()
					return err
				}
				entryInfo, infoErr := entry.Info()
				if infoErr != nil {
					_ = directory.Close()
					return infoErr
				}
				if entryInfo.Mode()&os.ModeSymlink != 0 || !entryInfo.IsDir() {
					continue
				}
				if len(stack)+len(w.watched) >= w.config.MaxWatches {
					_ = directory.Close()
					return fmt.Errorf("%w: watch count exceeded %d", ErrWatchBudget, w.config.MaxWatches)
				}
				stack = append(stack, filepath.Join(path, entry.Name()))
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				_ = directory.Close()
				return readErr
			}
		}
		if err := directory.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) consumeScanBudget(budget *scanBudget) error {
	budget.entries++
	if budget.entries > w.config.MaxScanEntries {
		return fmt.Errorf("%w: traversal exceeded %d entries", ErrWatchBudget, w.config.MaxScanEntries)
	}
	if !w.now().Before(budget.deadline) {
		return fmt.Errorf("%w: traversal exceeded %s", ErrWatchBudget, w.config.ScanTimeout)
	}
	return nil
}

func (w *Watcher) addDirectory(target backend, path string) error {
	if _, ok := w.watched[path]; ok {
		return nil
	}
	if len(w.watched) >= w.config.MaxWatches {
		return fmt.Errorf("%w: watch count exceeded %d", ErrWatchBudget, w.config.MaxWatches)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil
	}
	if err := target.Add(path); err != nil {
		return err
	}
	w.watched[path] = struct{}{}
	return nil
}

func pathWithin(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && !filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func sendSignal(ctx context.Context, output chan<- Signal, signal Signal) {
	select {
	case output <- signal:
	case <-ctx.Done():
	}
}

func waitForRetry(ctx context.Context, clock Clock, duration time.Duration) bool {
	timer := clock.NewTimer(duration)
	defer stopTimer(timer)
	select {
	case <-timer.C():
		return true
	case <-ctx.Done():
		return false
	}
}
