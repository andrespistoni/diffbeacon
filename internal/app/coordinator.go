package app

import (
	"context"
	"sync"

	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/highlight"
	watchpkg "github.com/andrespistoni/diffbeacon/internal/watch"
)

type RefreshRequest struct {
	Reason       RefreshReason
	Selection    gitpkg.ChangeIdentity
	HasSelection bool
}

type RefreshPayload struct {
	Snapshot gitpkg.Snapshot
	Detail   Detail
}

type RefreshLoader interface {
	Load(context.Context, RefreshRequest) (RefreshPayload, error)
}

type Coordinator struct {
	loader RefreshLoader
	output chan RefreshCompleted

	mu         sync.Mutex
	generation uint64
	cancel     context.CancelFunc
	closed     bool
	root       context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup
}

func NewCoordinator(loader RefreshLoader) *Coordinator {
	root, cancel := context.WithCancel(context.Background())
	coordinator := &Coordinator{
		loader: loader, output: make(chan RefreshCompleted, 16),
		root: root, rootCancel: cancel,
	}
	return coordinator
}

func (c *Coordinator) Results() <-chan RefreshCompleted { return c.output }

// Begin starts replacement work and returns the synchronous state transition.
// No Git, diff or highlighting work is performed on the caller's goroutine.
func (c *Coordinator) Begin(request RefreshRequest) RefreshStarted {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return RefreshStarted{Generation: c.generation, Reason: request.Reason}
	}
	c.generation++
	if c.cancel != nil {
		c.cancel()
	}
	ctx, cancel := context.WithCancel(c.root)
	c.cancel = cancel
	generation := c.generation
	c.wg.Add(1)
	go c.run(ctx, generation, request)
	return RefreshStarted{Generation: generation, Reason: request.Reason}
}

// ConsumeWatch bridges watcher notifications to refresh work. It runs in its
// own goroutine; callers apply the returned RefreshStarted messages in their
// interaction loop.
func (c *Coordinator) ConsumeWatch(ctx context.Context, signals <-chan watchpkg.Signal, request func() RefreshRequest) <-chan RefreshStarted {
	started := make(chan RefreshStarted, 16)
	go func() {
		defer close(started)
		for {
			select {
			case <-ctx.Done():
				return
			case signal, ok := <-signals:
				if !ok {
					return
				}
				refresh := request()
				if signal.Reason == watchpkg.ReasonReconcile {
					refresh.Reason = RefreshReconcile
				} else {
					refresh.Reason = RefreshWatch
				}
				message := c.Begin(refresh)
				select {
				case started <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return started
}

func (c *Coordinator) run(ctx context.Context, generation uint64, request RefreshRequest) {
	defer c.wg.Done()
	payload, err := c.loader.Load(ctx, request)
	result := RefreshCompleted{Generation: generation, Snapshot: payload.Snapshot, Detail: payload.Detail, Err: err}
	select {
	case c.output <- result:
	case <-c.root.Done():
	}
}

func (c *Coordinator) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	c.rootCancel()
	c.mu.Unlock()
	c.wg.Wait()
	close(c.output)
}

type GitLoader struct {
	Runner          *gitpkg.Runner
	Repository      gitpkg.Repository
	DiffLimits      diffpkg.Limits
	HighlightLimits highlight.Limits
}

func (l GitLoader) Load(ctx context.Context, request RefreshRequest) (RefreshPayload, error) {
	snapshot, err := gitpkg.QueryStatus(ctx, l.Runner, l.Repository)
	if err != nil {
		return RefreshPayload{}, err
	}
	payload := RefreshPayload{Snapshot: snapshot}
	identity, ok := reconcileSelection(nil, snapshot.Changes, request.Selection, request.HasSelection)
	if !ok {
		return payload, nil
	}
	change, ok := changeByIdentity(snapshot.Changes, identity)
	if !ok {
		return payload, nil
	}
	payload.Detail.Identity = identity
	document, err := gitpkg.LoadContentWithLimits(ctx, l.Runner, l.Repository, change, l.DiffLimits)
	if err != nil {
		payload.Detail.Error = makeAppError("content load failed", err)
		return payload, nil
	}
	model := diffpkg.Build(document, l.DiffLimits)
	payload.Detail.Diff = &model
	payload.Detail.Highlight = highlight.Apply(ctx, change.Path, &model, l.HighlightLimits)
	return payload, nil
}

func changeByIdentity(changes []gitpkg.Change, identity gitpkg.ChangeIdentity) (gitpkg.Change, bool) {
	for _, change := range changes {
		if change.Identity() == identity {
			return change, true
		}
	}
	return gitpkg.Change{}, false
}
