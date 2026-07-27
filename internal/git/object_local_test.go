package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/testrepo"
)

func TestLocalObjectDetectionCoversLoosePackedAlternateAndWorktreeObjects(t *testing.T) {
	source := testrepo.New(t)
	source.Write("content.txt", "local object\n")
	source.CommitAll("local object")
	oid := source.Git("rev-parse", "HEAD:content.txt")
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, source.Path)
	if err != nil {
		t.Fatal(err)
	}
	assertContentLoads(t, runner, repository, source.Path)

	source.Git("gc", "--quiet", "--prune=now")
	assertContentLoads(t, runner, repository, source.Path)

	alternate := testrepo.New(t)
	alternateGitDir := alternate.GitDir()
	if err := os.MkdirAll(filepath.Join(alternateGitDir, "objects", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	value := filepath.Join(source.GitDir(), "objects") + "\n"
	if err := os.WriteFile(filepath.Join(alternateGitDir, "objects", "info", "alternates"), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
	alternateRepository, err := gitpkg.Discover(context.Background(), runner, alternate.Path)
	if err != nil {
		t.Fatal(err)
	}
	change := gitpkg.Change{Path: "content.txt", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusAdded}
	alternate.Git("update-index", "--add", "--cacheinfo", "100644,"+oid+",content.txt")
	if _, err := gitpkg.LoadContent(context.Background(), runner, alternateRepository, change); err != nil {
		t.Fatalf("load alternate packed object: %v", err)
	}

	worktreePath := filepath.Join(t.TempDir(), "linked")
	source.Git("worktree", "add", "--quiet", "--detach", worktreePath, "HEAD")
	worktreeRepository, err := gitpkg.Discover(context.Background(), runner, worktreePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gitpkg.LoadContent(context.Background(), runner, worktreeRepository, gitpkg.Change{Path: "content.txt", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusDeleted}); err != nil {
		t.Fatalf("load packed object through worktree commondir: %v", err)
	}
}

func assertContentLoads(t *testing.T, runner *gitpkg.Runner, repository gitpkg.Repository, root string) {
	t.Helper()
	change := gitpkg.Change{Path: "content.txt", Scope: gitpkg.ScopeStaged, Status: gitpkg.StatusDeleted}
	if _, err := gitpkg.LoadContent(context.Background(), runner, repository, change); err != nil {
		t.Fatalf("LoadContent(%s): %v", root, err)
	}
}
