package testrepo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	externalOperationEnv = "DIFFBEACON_TEST_EXTERNAL_OPERATION"
	externalPathEnv      = "DIFFBEACON_TEST_EXTERNAL_PATH"
)

// ExternalGit runs the system Git as a process isolated from the developer's
// configuration. It represents a repository mutation performed outside
// DiffBeacon while an integration test keeps the application alive.
func (r *Repository) ExternalGit(args ...string) string {
	r.t.Helper()
	output, err := r.ExternalGitInputError(nil, args...)
	if err != nil {
		r.t.Fatalf("external git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(output)
}

// ExternalGitError is ExternalGit's non-fatal variant for expected failures.
func (r *Repository) ExternalGitError(args ...string) (string, error) {
	return r.ExternalGitInputError(nil, args...)
}

// ExternalGitInput runs an external Git process with explicit standard input.
func (r *Repository) ExternalGitInput(input string, args ...string) string {
	r.t.Helper()
	output, err := r.ExternalGitInputError(strings.NewReader(input), args...)
	if err != nil {
		r.t.Fatalf("external git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(output)
}

func (r *Repository) ExternalGitInputError(input io.Reader, args ...string) (string, error) {
	r.t.Helper()
	command := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	command.Dir = r.Path
	command.Env = isolatedEnvironment(r.Home)
	command.Stdin = input
	output, err := command.CombinedOutput()
	return string(output), err
}

// ExternalWrite writes a working-tree file from a separate process. Tests that
// call this method must expose TestDiffBeaconExternalProcess and delegate it to
// RunExternalProcess.
func (r *Repository) ExternalWrite(path, content string) {
	r.t.Helper()
	r.runExternalFileOperation("write", path, strings.NewReader(content))
}

// ExternalRemove removes a working-tree path from a separate process.
func (r *Repository) ExternalRemove(path string) {
	r.t.Helper()
	r.runExternalFileOperation("remove", path, nil)
}

func (r *Repository) runExternalFileOperation(operation, path string, input io.Reader) {
	r.t.Helper()
	fullPath := filepath.Join(r.Path, filepath.FromSlash(path))
	relative, err := filepath.Rel(r.Path, fullPath)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		r.t.Fatalf("external %s path %q escapes repository", operation, path)
	}
	executable, err := os.Executable()
	if err != nil {
		r.t.Fatalf("locate test executable: %v", err)
	}
	command := exec.Command(executable, "-test.run=^TestDiffBeaconExternalProcess$")
	command.Env = append(os.Environ(), externalOperationEnv+"="+operation, externalPathEnv+"="+fullPath)
	command.Stdin = input
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		r.t.Fatalf("external %s %q: %v\n%s", operation, path, err, output.String())
	}
}

// RunExternalProcess handles the re-executed test process used by
// ExternalWrite and ExternalRemove. It returns handled=false in the parent.
func RunExternalProcess() (handled bool, err error) {
	operation := os.Getenv(externalOperationEnv)
	if operation == "" {
		return false, nil
	}
	path := os.Getenv(externalPathEnv)
	if path == "" {
		return true, fmt.Errorf("external helper path is empty")
	}
	switch operation {
	case "write":
		content, readErr := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
		if readErr != nil {
			return true, readErr
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return true, err
		}
		return true, os.WriteFile(path, content, 0o644)
	case "remove":
		return true, os.Remove(path)
	default:
		return true, fmt.Errorf("unknown external helper operation %q", operation)
	}
}
