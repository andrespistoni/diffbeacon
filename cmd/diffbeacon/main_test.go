package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "diffbeacon/internal/git"
	"diffbeacon/internal/testrepo"
)

func TestRunDiscoversRepository(t *testing.T) {
	repository := testrepo.New(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{repository.Path}, strings.NewReader("q"), &stdout, &stderr, gitpkg.NewRunner("git")); code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[?1049h") || !strings.Contains(stdout.String(), "\x1b[?1049l") {
		t.Fatalf("stdout = %q, want alternate-screen enter and exit", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsMultiplePaths(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"one", "two"}, strings.NewReader("q"), &stdout, &stderr, gitpkg.NewRunner("git")); code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if got, want := stderr.String(), "usage: diffbeacon [path]\n       diffbeacon --version\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReportsVersionWithoutGitOrRepository(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	previous := version
	version = "0.1.0-test"
	t.Cleanup(func() { version = previous })

	if code := run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr, gitpkg.NewRunner(filepath.Join(t.TempDir(), "missing-git"))); code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if got, want := stdout.String(), "diffbeacon 0.1.0-test\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReportsRepositoryFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := gitpkg.NewRunner(filepath.Join(t.TempDir(), "missing-git"))

	if code := run(context.Background(), []string{t.TempDir()}, strings.NewReader("q"), &stdout, &stderr, runner); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), gitpkg.ErrGitUnavailable.Error()) {
		t.Fatalf("stderr = %q, want unavailable Git diagnosis", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !errors.Is(extractDiscoveryError(context.Background(), runner, t.TempDir()), gitpkg.ErrGitUnavailable) {
		t.Fatal("test setup did not produce ErrGitUnavailable")
	}
}

func TestRunSanitizesHostileDiscoveryPathBeforeTUI(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	hostile := filepath.Join(t.TempDir(), "osc\x1b]52;c;payload\x07-csi\x1b[31m-c1\u009b32m")

	if code := run(context.Background(), []string{hostile}, strings.NewReader("q"), &stdout, &stderr, gitpkg.NewRunner("git")); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	got := stderr.String()
	for _, control := range []rune{'\x1b', '\x07', '\u009b'} {
		if strings.ContainsRune(got, control) {
			t.Fatalf("stderr retained terminal control U+%04X: %q", control, got)
		}
	}
	if !strings.Contains(got, `\x1b]52`) || !strings.Contains(got, `\x1b[31m`) || !strings.Contains(got, `\u009b32m`) {
		t.Fatalf("stderr = %q, want visible OSC/CSI controls", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no TUI", stdout.String())
	}
}

func TestRunChecksGitCompatibilityBeforeOpeningTUI(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := gitpkg.NewRunner(filepath.Join(t.TempDir(), "missing-git"))

	if code := run(context.Background(), nil, strings.NewReader("q"), &stdout, &stderr, runner); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), gitpkg.ErrGitUnavailable.Error()) {
		t.Fatalf("stderr = %q, want Git startup diagnosis", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no TUI output", stdout.String())
	}
}

func TestRunRejectsIncompatibleGitBeforeOpeningTUI(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	gitPath := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\nprintf 'git version 2.30.9\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code := run(context.Background(), nil, strings.NewReader("q"), &stdout, &stderr, gitpkg.NewRunner(gitPath)); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), gitpkg.ErrGitIncompatible.Error()) || !strings.Contains(stderr.String(), gitpkg.MinimumVersion.String()) {
		t.Fatalf("stderr = %q, want explicit incompatibility and minimum", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no TUI output", stdout.String())
	}
}

func extractDiscoveryError(ctx context.Context, runner *gitpkg.Runner, path string) error {
	_, err := gitpkg.Discover(ctx, runner, path)
	return err
}
