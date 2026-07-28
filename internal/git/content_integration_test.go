package git_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestScopeContentUsesDistinctHEADIndexAndWorkingTree(t *testing.T) {
	path := "line\nunicodé -triple.txt"
	repository := testrepo.NewTripleContent(t, path)
	discovered, snapshot := contentSnapshot(t, repository)
	runner := gitpkg.NewRunner("git")

	staged := mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: path, Scope: gitpkg.ScopeStaged})
	stagedDocument, err := gitpkg.LoadContent(context.Background(), runner, discovered, staged)
	if err != nil {
		t.Fatalf("LoadContent(staged) error = %v", err)
	}
	if stagedDocument.Before != "head\n" || stagedDocument.After != "index\n" {
		t.Fatalf("staged document = %#v", stagedDocument)
	}

	unstaged := mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: path, Scope: gitpkg.ScopeUnstaged})
	unstagedDocument, err := gitpkg.LoadContent(context.Background(), runner, discovered, unstaged)
	if err != nil {
		t.Fatalf("LoadContent(unstaged) error = %v", err)
	}
	if unstagedDocument.Before != "index\n" || unstagedDocument.After != "working tree\n" {
		t.Fatalf("unstaged document = %#v", unstagedDocument)
	}
}

func TestContentUntrackedAndDeletedUseCompleteApplicableSide(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("deleted.txt", "old\ncontent\n")
	repository.CommitAll("base")
	repository.Remove("deleted.txt")
	repository.Write("-untracked name.txt", "new\ncontent\n")
	discovered, snapshot := contentSnapshot(t, repository)
	runner := gitpkg.NewRunner("git")

	deleted, err := gitpkg.LoadContent(context.Background(), runner, discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "deleted.txt", Scope: gitpkg.ScopeUnstaged}))
	if err != nil {
		t.Fatalf("LoadContent(deleted) error = %v", err)
	}
	if deleted.Before != "old\ncontent\n" || deleted.AfterPresent || !deleted.BeforePresent {
		t.Fatalf("deleted document = %#v", deleted)
	}

	untracked, err := gitpkg.LoadContent(context.Background(), runner, discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "-untracked name.txt", Scope: gitpkg.ScopeUntracked}))
	if err != nil {
		t.Fatalf("LoadContent(untracked) error = %v", err)
	}
	if untracked.BeforePresent || untracked.After != "new\ncontent\n" || untracked.Capability.Hunks {
		t.Fatalf("untracked document = %#v", untracked)
	}
}

func TestRenamedContentUsesOldPathAsItsBaseline(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("old.txt", "one\ntwo\nthree\nfour\n")
	repository.CommitAll("base")
	repository.Rename("old.txt", "new.txt")
	repository.Write("new.txt", "one\nchanged\nthree\nfour\n")
	repository.Git("add", "-A")
	discovered, snapshot := contentSnapshot(t, repository)
	change := mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "new.txt", Scope: gitpkg.ScopeStaged})
	if change.Status != gitpkg.StatusRenamed || change.OldPath != "old.txt" {
		t.Fatalf("rename change = %#v", change)
	}
	document, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, change)
	if err != nil {
		t.Fatalf("LoadContent(rename) error = %v", err)
	}
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	if model.Degraded || len(model.Hunks) != 1 || document.Before != "one\ntwo\nthree\nfour\n" || document.After != "one\nchanged\nthree\nfour\n" {
		t.Fatalf("rename model = %#v document = %#v", model, document)
	}
}

func TestSpecialContentBinaryAndLimitNeverExposeBytesAsText(t *testing.T) {
	t.Run("binary", func(t *testing.T) {
		repository := testrepo.New(t)
		repository.Write("binary.dat", "safe\x00\x1b[31munsafe")
		discovered, snapshot := contentSnapshot(t, repository)
		document, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "binary.dat", Scope: gitpkg.ScopeUntracked}))
		if err != nil {
			t.Fatalf("LoadContent(binary) error = %v", err)
		}
		if document.Kind != diffpkg.ContentBinary || document.After != "" || document.Capability.Hunks {
			t.Fatalf("binary document = %#v", document)
		}
	})

	t.Run("large", func(t *testing.T) {
		repository := testrepo.New(t)
		repository.Write("large.txt", strings.Repeat("x", diffpkg.DefaultMaxContentBytes+1))
		discovered, snapshot := contentSnapshot(t, repository)
		document, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "large.txt", Scope: gitpkg.ScopeUntracked}))
		if err != nil {
			t.Fatalf("LoadContent(large) error = %v", err)
		}
		if document.Kind != diffpkg.ContentLimited || document.After != "" || !strings.Contains(document.Capability.FullFileReason, "1048576") {
			t.Fatalf("limited document = %#v", document)
		}
	})
}

func TestSpecialContentConflictAndSubmoduleDisableHunks(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		repository := testrepo.NewConflict(t, "conflict.txt")
		discovered, snapshot := contentSnapshot(t, repository)
		document, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "conflict.txt", Scope: gitpkg.ScopeUnstaged}))
		if err != nil {
			t.Fatalf("LoadContent(conflict) error = %v", err)
		}
		if document.Kind != diffpkg.ContentConflict || document.Capability.Hunks || !strings.Contains(document.After, "<<<<<<<") {
			t.Fatalf("conflict document = %#v", document)
		}
	})

	t.Run("submodule", func(t *testing.T) {
		if os.Getenv("CI") != "" && runtime.GOOS == "windows" {
			t.Skip("local file submodule fixture is Unix-only in this batch")
		}
		repository, path := testrepo.NewSubmoduleChange(t)
		discovered, snapshot := contentSnapshot(t, repository)
		document, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: path, Scope: gitpkg.ScopeUnstaged}))
		if err != nil {
			t.Fatalf("LoadContent(submodule) error = %v", err)
		}
		if document.Kind != diffpkg.ContentSubmodule || document.Capability.Hunks || !strings.Contains(document.Metadata.Summary, "submodule") {
			t.Fatalf("submodule document = %#v", document)
		}
	})
}

func TestSpecialContentDoesNotFollowUntrackedSymlinkOutsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix semantics")
	}
	repository := testrepo.New(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repository.Path, "outside-link")); err != nil {
		t.Fatal(err)
	}
	discovered, snapshot := contentSnapshot(t, repository)
	_, err := gitpkg.LoadContent(context.Background(), gitpkg.NewRunner("git"), discovered, mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "outside-link", Scope: gitpkg.ScopeUntracked}))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadContent(symlink) error = %v, want explicit safe-read failure", err)
	}
}

func TestLargeTrackedFileKeepsSmallChangesWhenFullFileIsUnavailable(t *testing.T) {
	repository := testrepo.New(t)
	lines := make([]string, 70_000)
	for index := range lines {
		lines[index] = fmt.Sprintf("line-%05d stable content", index+1)
	}
	before := strings.Join(lines, "\n") + "\n"
	repository.Write("large.txt", before)
	repository.CommitAll("base")
	lines[len(lines)/2] = "line-35001 changed content"
	repository.Write("large.txt", strings.Join(lines, "\n")+"\n")

	assertChangesOnly := func(scope gitpkg.Scope) {
		t.Helper()
		discovered, snapshot := contentSnapshot(t, repository)
		change := mustContentChange(t, snapshot, gitpkg.ChangeIdentity{Path: "large.txt", Scope: scope})
		document, err := gitpkg.LoadContentWithLimits(context.Background(), gitpkg.NewRunner("git"), discovered, change, diffpkg.DefaultLimits())
		if err != nil {
			t.Fatalf("LoadContentWithLimits(%s) error = %v", scope, err)
		}
		if document.Kind != diffpkg.ContentText || document.Capability.FullFile || !document.Capability.Hunks || document.Patch == "" {
			t.Fatalf("%s document capabilities = %#v", scope, document)
		}
		model := diffpkg.Build(document, diffpkg.DefaultLimits())
		if model.Degraded || len(model.Hunks) != 1 || len(model.FullRows) != 0 {
			t.Fatalf("%s model = degraded %v reason %q hunks %d full rows %d", scope, model.Degraded, model.Reason, len(model.Hunks), len(model.FullRows))
		}
	}

	assertChangesOnly(gitpkg.ScopeUnstaged)
	repository.Git("add", "--", "large.txt")
	assertChangesOnly(gitpkg.ScopeStaged)
}

func contentSnapshot(t *testing.T, repository *testrepo.Repository) (gitpkg.Repository, gitpkg.Snapshot) {
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
	return discovered, snapshot
}

func mustContentChange(t *testing.T, snapshot gitpkg.Snapshot, identity gitpkg.ChangeIdentity) gitpkg.Change {
	t.Helper()
	for _, change := range snapshot.Changes {
		if change.Identity() == identity {
			return change
		}
	}
	t.Fatalf("missing change %#v in %#v", identity, snapshot.Changes)
	return gitpkg.Change{}
}
