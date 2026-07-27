package app

import (
	gitpkg "diffbeacon/internal/git"
)

type Message interface {
	isAppMessage()
}

type RefreshReason string

const (
	RefreshInitial   RefreshReason = "initial"
	RefreshManual    RefreshReason = "manual"
	RefreshWatch     RefreshReason = "watch"
	RefreshReconcile RefreshReason = "reconcile"
)

type RefreshStarted struct {
	Generation uint64
	Reason     RefreshReason
}

func (RefreshStarted) isAppMessage() {}

type RefreshCompleted struct {
	Generation uint64
	Snapshot   gitpkg.Snapshot
	Detail     Detail
	Err        error
}

func (RefreshCompleted) isAppMessage() {}

type SelectChange struct {
	Identity gitpkg.ChangeIdentity
}

func (SelectChange) isAppMessage() {}

type SetFilter struct{ Filter Filter }

func (SetFilter) isAppMessage() {}

type SetActiveHunk struct{ HunkID string }

func (SetActiveHunk) isAppMessage() {}

type SetScroll struct{ Vertical, Horizontal int }

func (SetScroll) isAppMessage() {}

type SetLayout struct{ Layout Layout }

func (SetLayout) isAppMessage() {}

type SetDensity struct{ Density Density }

func (SetDensity) isAppMessage() {}

type SetFocus struct{ Focus Focus }

func (SetFocus) isAppMessage() {}
