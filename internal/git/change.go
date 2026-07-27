package git

import "fmt"

type Scope uint8

const (
	ScopeStaged Scope = iota + 1
	ScopeUnstaged
	ScopeUntracked
)

func (s Scope) String() string {
	switch s {
	case ScopeStaged:
		return "staged"
	case ScopeUnstaged:
		return "unstaged"
	case ScopeUntracked:
		return "untracked"
	default:
		return fmt.Sprintf("scope(%d)", s)
	}
}

func (s Scope) Symbol() string {
	switch s {
	case ScopeStaged:
		return "S"
	case ScopeUnstaged:
		return "U"
	case ScopeUntracked:
		return "?"
	default:
		return "!"
	}
}

type ChangeStatus uint8

const (
	StatusAdded ChangeStatus = iota + 1
	StatusModified
	StatusDeleted
	StatusRenamed
	StatusCopied
	StatusTypeChanged
	StatusUntracked
	StatusUnmerged
)

func (s ChangeStatus) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	case StatusTypeChanged:
		return "type changed"
	case StatusUntracked:
		return "untracked"
	case StatusUnmerged:
		return "unmerged"
	default:
		return fmt.Sprintf("status(%d)", s)
	}
}

func (s ChangeStatus) Symbol() string {
	switch s {
	case StatusAdded:
		return "A"
	case StatusModified:
		return "M"
	case StatusDeleted:
		return "D"
	case StatusRenamed:
		return "R"
	case StatusCopied:
		return "C"
	case StatusTypeChanged:
		return "T"
	case StatusUntracked:
		return "?"
	case StatusUnmerged:
		return "U"
	default:
		return "!"
	}
}

type ConflictKind string

const (
	ConflictBothDeleted   ConflictKind = "DD"
	ConflictAddedByUs     ConflictKind = "AU"
	ConflictDeletedByThem ConflictKind = "UD"
	ConflictAddedByThem   ConflictKind = "UA"
	ConflictDeletedByUs   ConflictKind = "DU"
	ConflictBothAdded     ConflictKind = "AA"
	ConflictBothModified  ConflictKind = "UU"
)

func (c ConflictKind) String() string {
	switch c {
	case ConflictBothDeleted:
		return "both deleted"
	case ConflictAddedByUs:
		return "added by us"
	case ConflictDeletedByThem:
		return "deleted by them"
	case ConflictAddedByThem:
		return "added by them"
	case ConflictDeletedByUs:
		return "deleted by us"
	case ConflictBothAdded:
		return "both added"
	case ConflictBothModified:
		return "both modified"
	case "":
		return "none"
	default:
		return string(c)
	}
}

type SubmoduleState struct {
	IsSubmodule      bool
	CommitChanged    bool
	TrackedModified  bool
	UntrackedPresent bool
}

type Change struct {
	Path      string
	OldPath   string
	Scope     Scope
	Status    ChangeStatus
	Conflict  ConflictKind
	Submodule SubmoduleState
}

type ChangeIdentity struct {
	Path  string
	Scope Scope
}

func (c Change) Identity() ChangeIdentity {
	return ChangeIdentity{Path: c.Path, Scope: c.Scope}
}

type Revision struct {
	HeadOID string
	Branch  string
}

func (r Revision) Valid() bool {
	return r.HeadOID != "" && r.Branch != ""
}

type Snapshot struct {
	Revision Revision
	Changes  []Change
}

func (s Snapshot) Count(scope Scope) int {
	count := 0
	for _, change := range s.Changes {
		if change.Scope == scope {
			count++
		}
	}
	return count
}
