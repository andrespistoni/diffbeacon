package app

import (
	"fmt"

	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/highlight"
)

type Filter uint8

const (
	FilterAll Filter = iota + 1
	FilterStaged
	FilterChanges
	FilterUntracked
)

type Layout uint8

const (
	LayoutInline Layout = iota + 1
	LayoutSideBySide
)

type Density uint8

const (
	DensityChanges Density = iota + 1
	DensityFullFile
)

type Focus uint8

const (
	FocusChanges Focus = iota + 1
	FocusContent
)

type Progress struct {
	Refreshing bool
	Generation uint64
	Reason     RefreshReason
}

type AppError struct {
	Summary string
	Detail  string
}

type Detail struct {
	Identity  gitpkg.ChangeIdentity
	Diff      *diffpkg.Model
	Highlight highlight.Result
	Error     *AppError
}

type Model struct {
	Snapshot gitpkg.Snapshot
	Filter   Filter
	Layout   Layout
	Density  Density
	Focus    Focus

	Selection    gitpkg.ChangeIdentity
	HasSelection bool
	ActiveHunkID string
	ScrollY      int
	ScrollX      int
	Detail       Detail
	Progress     Progress
	Error        *AppError
}

func NewModel() Model {
	return Model{
		Filter:  FilterAll,
		Layout:  LayoutInline,
		Density: DensityChanges,
		Focus:   FocusChanges,
	}
}

func (m Model) VisibleChanges() []gitpkg.Change {
	return visibleChanges(m.Snapshot.Changes, m.Filter)
}

func visibleChanges(changes []gitpkg.Change, filter Filter) []gitpkg.Change {
	result := make([]gitpkg.Change, 0, len(changes))
	for _, change := range changes {
		if filterMatches(filter, change.Scope) {
			result = append(result, change)
		}
	}
	return result
}

func filterMatches(filter Filter, scope gitpkg.Scope) bool {
	switch filter {
	case FilterAll:
		return true
	case FilterStaged:
		return scope == gitpkg.ScopeStaged
	case FilterChanges:
		return scope == gitpkg.ScopeUnstaged || scope == gitpkg.ScopeUntracked
	case FilterUntracked:
		return scope == gitpkg.ScopeUntracked
	default:
		return false
	}
}

func filterForScope(scope gitpkg.Scope) Filter {
	switch scope {
	case gitpkg.ScopeStaged:
		return FilterStaged
	case gitpkg.ScopeUnstaged:
		return FilterChanges
	case gitpkg.ScopeUntracked:
		return FilterChanges
	default:
		return FilterAll
	}
}

func makeAppError(summary string, err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{Summary: summary, Detail: err.Error()}
}

func (e AppError) Error() string {
	if e.Detail == "" {
		return e.Summary
	}
	return fmt.Sprintf("%s: %s", e.Summary, e.Detail)
}
