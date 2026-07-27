package git

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	DefaultStatusMaxBytes   = 8 * 1024 * 1024
	DefaultStatusMaxEntries = 100_000
	DefaultStatusTimeout    = 2 * time.Second
)

var ErrStatusBudget = errors.New("Git status exceeded its resource budget; showing the previous snapshot until reconciliation succeeds")

type statusLimits struct {
	maxBytes   int
	maxEntries int
	timeout    time.Duration
}

var defaultStatusLimits = statusLimits{maxBytes: DefaultStatusMaxBytes, maxEntries: DefaultStatusMaxEntries, timeout: DefaultStatusTimeout}

// QueryStatus obtains one porcelain-v2 snapshot from Git and classifies it by scope.
func QueryStatus(ctx context.Context, runner *Runner, repository Repository) (Snapshot, error) {
	return queryStatusWithLimits(ctx, runner, repository, defaultStatusLimits)
}

func queryStatusWithLimits(ctx context.Context, runner *Runner, repository Repository, limits statusLimits) (Snapshot, error) {
	if limits.maxBytes <= 0 || limits.maxEntries <= 0 || limits.timeout <= 0 {
		limits = defaultStatusLimits
	}
	queryCtx, cancel := context.WithTimeout(ctx, limits.timeout)
	defer cancel()
	output, truncated, err := runner.RunLimited(
		queryCtx,
		repository.Root,
		limits.maxBytes,
		"--no-optional-locks",
		"-c", "color.status=false",
		"-c", "core.quotepath=false",
		"-c", "status.renames=copies",
		"status",
		"--porcelain=v2",
		"-z",
		"--branch",
		"--no-ahead-behind",
		"--untracked-files=all",
		"--ignored=no",
		"--ignore-submodules=none",
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return Snapshot{}, fmt.Errorf("%w: query exceeded %s", ErrStatusBudget, limits.timeout)
		}
		return Snapshot{}, fmt.Errorf("query Git status: %w", err)
	}
	if truncated {
		return Snapshot{}, fmt.Errorf("%w: porcelain output exceeded %d bytes", ErrStatusBudget, limits.maxBytes)
	}

	snapshot, err := parseStatusLimited(output, limits.maxEntries)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse Git status: %w", err)
	}
	if !snapshot.Revision.Valid() {
		return Snapshot{}, fmt.Errorf("parse Git status: missing branch revision headers")
	}
	sort.SliceStable(snapshot.Changes, func(left, right int) bool {
		if snapshot.Changes[left].Path != snapshot.Changes[right].Path {
			return snapshot.Changes[left].Path < snapshot.Changes[right].Path
		}
		if snapshot.Changes[left].Scope != snapshot.Changes[right].Scope {
			return snapshot.Changes[left].Scope < snapshot.Changes[right].Scope
		}
		return snapshot.Changes[left].Status < snapshot.Changes[right].Status
	})
	return snapshot, nil
}
