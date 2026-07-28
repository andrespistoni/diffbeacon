package git_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestCleanRepository(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("tracked.txt", "clean\n")
	repository.CommitAll("clean")

	snapshot := queryStatus(t, repository)
	if !snapshot.Revision.Valid() || snapshot.Revision.HeadOID == "(initial)" {
		t.Fatalf("Revision = %#v, want committed revision", snapshot.Revision)
	}
	if len(snapshot.Changes) != 0 {
		t.Fatalf("Changes = %#v, want clean snapshot", snapshot.Changes)
	}
}

func TestStatusIntegrationClassifiesScopesAndPreservesPaths(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write(".gitignore", "ignored.log\n")
	repository.Write("double.txt", "base\n")
	repository.CommitAll("base")

	repository.Write("double.txt", "staged\n")
	repository.Git("add", "--", "double.txt")
	repository.Write("double.txt", "worktree\n")

	hostilePaths := []string{
		"path with spaces.txt",
		"unicodé-文件.txt",
		"line\nbreak.txt",
		"$special;[chars]*.txt",
		"-leading-option.txt",
	}
	for _, path := range hostilePaths {
		repository.Write(path, "untracked\n")
	}
	repository.Write("ignored.log", "not visible\n")

	snapshot := queryStatus(t, repository)
	changes := changesByIdentity(snapshot.Changes)
	for _, identity := range []gitpkg.ChangeIdentity{
		{Path: "double.txt", Scope: gitpkg.ScopeStaged},
		{Path: "double.txt", Scope: gitpkg.ScopeUnstaged},
	} {
		if _, ok := changes[identity]; !ok {
			t.Errorf("missing double-scope identity %#v; changes = %#v", identity, snapshot.Changes)
		}
	}
	for _, path := range hostilePaths {
		identity := gitpkg.ChangeIdentity{Path: path, Scope: gitpkg.ScopeUntracked}
		if _, ok := changes[identity]; !ok {
			t.Errorf("missing hostile path identity %#v", identity)
		}
	}
	for _, change := range snapshot.Changes {
		if change.Path == "ignored.log" {
			t.Fatalf("ignored path appeared in status: %#v", change)
		}
	}
}

func TestStatusIntegrationRepositoryWithoutHEAD(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("first.txt", "first\n")
	repository.Git("add", "--", "first.txt")

	snapshot := queryStatus(t, repository)
	if snapshot.Revision.HeadOID != "(initial)" || snapshot.Revision.Branch != "main" {
		t.Fatalf("Revision = %#v, want initial main", snapshot.Revision)
	}
	assertChange(t, snapshot.Changes, gitpkg.ChangeIdentity{Path: "first.txt", Scope: gitpkg.ScopeStaged}, gitpkg.StatusAdded)
}

func TestStatusIntegrationRenameDeleteAndTypeChange(t *testing.T) {
	t.Run("rename", func(t *testing.T) {
		repository := testrepo.New(t)
		repository.Write("old name.txt", "same\n")
		repository.CommitAll("base")
		repository.Rename("old name.txt", "new name.txt")
		repository.Git("add", "-A")

		change := assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: "new name.txt", Scope: gitpkg.ScopeStaged}, gitpkg.StatusRenamed)
		if change.OldPath != "old name.txt" {
			t.Fatalf("OldPath = %q, want %q", change.OldPath, "old name.txt")
		}
	})

	t.Run("copy when enabled in Git", func(t *testing.T) {
		repository := testrepo.New(t)
		base := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
		repository.Write("source.txt", base)
		repository.CommitAll("base")
		repository.Git("config", "status.renames", "copies")
		repository.Write("source.txt", base+"changed\n")
		repository.Write("copy.txt", base+"changed\n")
		repository.Git("add", "--", "source.txt", "copy.txt")

		change := assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: "copy.txt", Scope: gitpkg.ScopeStaged}, gitpkg.StatusCopied)
		if change.OldPath != "source.txt" {
			t.Fatalf("OldPath = %q, want %q", change.OldPath, "source.txt")
		}
	})

	t.Run("delete", func(t *testing.T) {
		repository := testrepo.New(t)
		repository.Write("deleted.txt", "base\n")
		repository.CommitAll("base")
		repository.Remove("deleted.txt")

		assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: "deleted.txt", Scope: gitpkg.ScopeUnstaged}, gitpkg.StatusDeleted)
	})

	t.Run("type change", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink fixture requires Unix semantics")
		}
		repository := testrepo.New(t)
		repository.Write("typed", "base\n")
		repository.CommitAll("base")
		repository.ReplaceWithSymlink("typed", "target")

		assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: "typed", Scope: gitpkg.ScopeUnstaged}, gitpkg.StatusTypeChanged)
	})
}

func TestStatusIntegrationConflict(t *testing.T) {
	repository := testrepo.NewConflict(t, "conflict.txt")
	change := assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: "conflict.txt", Scope: gitpkg.ScopeUnstaged}, gitpkg.StatusUnmerged)
	if change.Conflict != gitpkg.ConflictBothModified {
		t.Fatalf("Conflict = %q, want %q", change.Conflict, gitpkg.ConflictBothModified)
	}
}

func TestStatusIntegrationSubmodule(t *testing.T) {
	if os.Getenv("CI") != "" && runtime.GOOS == "windows" {
		t.Skip("local file submodule fixture is Unix-only in this batch")
	}
	repository, path := testrepo.NewSubmoduleChange(t)
	change := assertChange(t, queryStatus(t, repository).Changes, gitpkg.ChangeIdentity{Path: path, Scope: gitpkg.ScopeUnstaged}, gitpkg.StatusModified)
	if !change.Submodule.IsSubmodule || !change.Submodule.TrackedModified {
		t.Fatalf("Submodule = %#v, want tracked modification", change.Submodule)
	}
}

func queryStatus(t *testing.T, repository *testrepo.Repository) gitpkg.Snapshot {
	t.Helper()
	runner := gitpkg.NewRunner("git")
	discovered, err := gitpkg.Discover(context.Background(), runner, repository.Path)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, discovered)
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	return snapshot
}

func changesByIdentity(changes []gitpkg.Change) map[gitpkg.ChangeIdentity]gitpkg.Change {
	result := make(map[gitpkg.ChangeIdentity]gitpkg.Change, len(changes))
	for _, change := range changes {
		result[change.Identity()] = change
	}
	return result
}

func assertChange(t *testing.T, changes []gitpkg.Change, identity gitpkg.ChangeIdentity, status gitpkg.ChangeStatus) gitpkg.Change {
	t.Helper()
	change, ok := changesByIdentity(changes)[identity]
	if !ok {
		t.Fatalf("missing change %#v; changes = %#v", identity, changes)
	}
	if change.Status != status {
		t.Fatalf("change %#v status = %v, want %v", identity, change.Status, status)
	}
	return change
}

func TestStatusIntegrationPathsRemainRepositoryRelative(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("nested/file.txt", "content\n")
	snapshot := queryStatus(t, repository)
	change := assertChange(t, snapshot.Changes, gitpkg.ChangeIdentity{Path: "nested/file.txt", Scope: gitpkg.ScopeUntracked}, gitpkg.StatusUntracked)
	if filepath.IsAbs(change.Path) {
		t.Fatalf("Path = %q, want repository-relative path", change.Path)
	}
}
