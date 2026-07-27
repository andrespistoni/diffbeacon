package git

import (
	"errors"
	"testing"
)

func TestReadOnlyPolicyAllowsProductionQueries(t *testing.T) {
	queries := [][]string{
		{"--version"},
		{"rev-parse", "--is-bare-repository"},
		{"rev-parse", "--show-toplevel", "--absolute-git-dir"},
		{"--no-optional-locks", "-c", "color.status=false", "-c", "core.quotepath=false", "-c", "status.renames=copies", "status", "--porcelain=v2", "-z", "--branch", "--no-ahead-behind", "--untracked-files=all", "--ignored=no", "--ignore-submodules=none"},
		{"--no-optional-locks", "--literal-pathspecs", "ls-tree", "-z", "HEAD", "--", "*.txt"},
		{"--no-optional-locks", "--literal-pathspecs", "ls-files", "--stage", "-z", "--", ":(glob)*.txt"},
		{"--no-optional-locks", "cat-file", "blob", "0123456789012345678901234567890123456789"},
		{"--no-optional-locks", "cat-file", "-s", "0123456789012345678901234567890123456789"},
		{"--no-optional-locks", "diff", "--patch", "--no-ext-diff", "--no-textconv", "--no-color", "--diff-algorithm=myers", "--unified=3", "--no-index", "--", "before", "after"},
	}
	for _, query := range queries {
		if err := validateReadOnlyGitInvocation(query); err != nil {
			t.Errorf("validateReadOnlyGitInvocation(%q) = %v", query, err)
		}
	}
}

func TestRejectMutation(t *testing.T) {
	mutations := [][]string{
		{"add", "--", "file"}, {"reset", "HEAD"}, {"rm", "--cached", "file"},
		{"read-tree", "--empty"}, {"apply", "--cached"}, {"checkout", "--", "file"},
		{"restore", "file"}, {"commit"}, {"update-ref", "HEAD", "deadbeef"},
		{"config", "user.name", "writer"}, {"gc"}, {"clean", "-fd"},
		{"status", "--porcelain=v2"}, {"cat-file", "--batch"},
		{"-c", "core.fsmonitor=true", "status"},
		{"-c", "protocol.allow=always", "rev-parse", "--is-bare-repository"},
		{"-c", "protocol.http.allow=always", "cat-file", "blob", "0123456789012345678901234567890123456789"},
		{"diff", "--patch", "--no-ext-diff", "--no-textconv", "--no-color", "--diff-algorithm=myers", "--unified=3", "--no-index", "--", "/etc/passwd", "after"},
	}
	for _, mutation := range mutations {
		if err := validateReadOnlyGitInvocation(mutation); !errors.Is(err, ErrGitCommandNotReadOnly) {
			t.Errorf("validateReadOnlyGitInvocation(%q) = %v, want ErrGitCommandNotReadOnly", mutation, err)
		}
	}
}
