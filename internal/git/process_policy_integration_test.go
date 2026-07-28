package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestRepositoryQueriesNeverExecuteConfiguredFSMonitor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell fixture is Unix-only")
	}
	repository := testrepo.New(t)
	repository.Write("tracked.txt", "base\n")
	repository.CommitAll("base")
	marker := filepath.Join(t.TempDir(), "fsmonitor-ran")
	repository.Git("config", "core.fsmonitor", writeMarkerHelper(t, marker))

	runner := gitpkg.NewRunner("git")
	discovered, err := gitpkg.Discover(context.Background(), runner, repository.Path)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if _, err := gitpkg.QueryStatus(context.Background(), runner, discovered); err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("configured fsmonitor executed or marker stat failed: %v", err)
	}
}

func TestStatusDoesNotExecuteConfiguredProcessFilter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell fixture is Unix-only")
	}
	repository := testrepo.New(t)
	repository.Write(".gitattributes", "filtered.txt filter=Hostile\n")
	repository.Write("filtered.txt", "base\n")
	repository.CommitAll("base")
	repository.Write("filtered.txt", "changed\n")
	marker := filepath.Join(t.TempDir(), "status-filter-ran")
	repository.Git("config", "filter.Hostile.process", writeMarkerHelper(t, marker))
	runner := gitpkg.NewRunner("git")
	discovered, err := gitpkg.Discover(context.Background(), runner, repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, discovered)
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("process filter executed during status or marker stat failed: %v", statErr)
	}
	if snapshot.Count(gitpkg.ScopeUnstaged) != 1 {
		t.Fatalf("status snapshot = %#v", snapshot.Changes)
	}
	document, err := gitpkg.LoadContent(context.Background(), runner, discovered, snapshot.Changes[0])
	if err != nil {
		t.Fatalf("LoadContent() error = %v", err)
	}
	if document.Patch == "" {
		t.Fatal("LoadContent() returned no textual patch")
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("process filter executed during diff or marker stat failed: %v", statErr)
	}
}

func writeMarkerHelper(t *testing.T, marker string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper")
	content := []byte("#!/bin/sh\ntouch " + shellSingleQuote(marker) + "\ncat\n")
	if err := os.WriteFile(path, content, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellSingleQuote(value string) string {
	result := "'"
	for _, character := range value {
		if character == '\'' {
			result += "'\\''"
		} else {
			result += string(character)
		}
	}
	return result + "'"
}
