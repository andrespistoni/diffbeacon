package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/testrepo"
)

func TestDiscoverFromRootSubdirectoryAndFile(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("nested/file.txt", "content\n")
	runner := gitpkg.NewRunner("git")

	paths := []string{
		repository.Path,
		filepath.Join(repository.Path, "nested"),
		filepath.Join(repository.Path, "nested", "file.txt"),
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			got, err := gitpkg.Discover(context.Background(), runner, path)
			if err != nil {
				t.Fatalf("Discover(%q) error = %v", path, err)
			}
			if got.Root != repository.Path {
				t.Fatalf("Root = %q, want %q", got.Root, repository.Path)
			}
			if !filepath.IsAbs(got.GitDir) {
				t.Fatalf("GitDir = %q, want absolute path", got.GitDir)
			}
		})
	}
}

func TestDiscoverLinkedWorktree(t *testing.T) {
	repository := testrepo.New(t)
	repository.Write("tracked.txt", "base\n")
	repository.CommitAll("base")
	worktree := filepath.Join(t.TempDir(), "linked")
	repository.Git("worktree", "add", "--quiet", "-b", "linked-test", worktree)

	gitFile, err := os.Stat(filepath.Join(worktree, ".git"))
	if err != nil {
		t.Fatalf("stat worktree .git: %v", err)
	}
	if !gitFile.Mode().IsRegular() {
		t.Fatalf("worktree .git mode = %v, want regular file", gitFile.Mode())
	}

	got, err := gitpkg.Discover(context.Background(), gitpkg.NewRunner("git"), worktree)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if got.Root != worktree {
		t.Fatalf("Root = %q, want %q", got.Root, worktree)
	}
	if got.GitDir == filepath.Join(worktree, ".git") {
		t.Fatalf("GitDir = %q, want resolved worktree Git directory", got.GitDir)
	}
}

func TestDiscoverRejectsOutsideRepository(t *testing.T) {
	_, err := gitpkg.Discover(context.Background(), gitpkg.NewRunner("git"), t.TempDir())
	if !errors.Is(err, gitpkg.ErrNotRepository) {
		t.Fatalf("Discover() error = %v, want ErrNotRepository", err)
	}
}

func TestDiscoverRejectsBareRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("Git is required for integration tests: %v", err)
	}
	bare := filepath.Join(t.TempDir(), "bare.git")
	cmd := exec.Command("git", "init", "--quiet", "--bare", bare)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, output)
	}

	_, err := gitpkg.Discover(context.Background(), gitpkg.NewRunner("git"), bare)
	if !errors.Is(err, gitpkg.ErrBareRepository) {
		t.Fatalf("Discover() error = %v, want ErrBareRepository", err)
	}
}

func TestDiscoverReportsUnavailableGit(t *testing.T) {
	_, err := gitpkg.Discover(context.Background(), gitpkg.NewRunner(filepath.Join(t.TempDir(), "missing-git")), t.TempDir())
	if !errors.Is(err, gitpkg.ErrGitUnavailable) {
		t.Fatalf("Discover() error = %v, want ErrGitUnavailable", err)
	}
}

func TestDiscoverReportsNonExecutableGit(t *testing.T) {
	program := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(program, []byte("not executable"), 0o644); err != nil {
		t.Fatalf("write fake Git: %v", err)
	}
	_, err := gitpkg.Discover(context.Background(), gitpkg.NewRunner(program), t.TempDir())
	if !errors.Is(err, gitpkg.ErrGitUnavailable) {
		t.Fatalf("Discover() error = %v, want ErrGitUnavailable", err)
	}
}
