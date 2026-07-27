package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	ErrGitUnavailable = errors.New("git is unavailable")
	ErrNotRepository  = errors.New("path is not in a Git repository")
	ErrBareRepository = errors.New("bare repositories are not supported")
)

type Repository struct {
	Root   string
	GitDir string
}

func Discover(ctx context.Context, runner *Runner, path string) (Repository, error) {
	if path == "" {
		path = "."
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return Repository{}, fmt.Errorf("resolve repository path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return Repository{}, fmt.Errorf("%w: %s: %v", ErrNotRepository, path, err)
	}
	directory := absolutePath
	if !info.IsDir() {
		directory = filepath.Dir(absolutePath)
	}

	bareOutput, err := runner.Run(ctx, directory, "rev-parse", "--is-bare-repository")
	if err != nil {
		return Repository{}, classifyDiscoveryError(err)
	}
	if strings.TrimSpace(string(bareOutput)) == "true" {
		return Repository{}, ErrBareRepository
	}

	output, err := runner.Run(ctx, directory, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		return Repository{}, classifyDiscoveryError(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 || lines[0] == "" || lines[1] == "" {
		return Repository{}, fmt.Errorf("%w: Git returned incomplete repository metadata", ErrNotRepository)
	}
	return Repository{Root: filepath.Clean(lines[0]), GitDir: filepath.Clean(lines[1])}, nil
}

func classifyDiscoveryError(err error) error {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.EACCES) {
		return fmt.Errorf("%w: %v", ErrGitUnavailable, err)
	}
	return fmt.Errorf("%w: %v", ErrNotRepository, err)
}
