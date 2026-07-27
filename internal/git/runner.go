package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"diffbeacon/internal/sanitize"
)

const (
	defaultStderrLimit = 8 * 1024
)

// Runner executes Git directly with structured arguments. It never invokes a shell.
type Runner struct {
	Program     string
	StderrLimit int

	prefixArgs []string
	extraEnv   []string
}

func NewRunner(program string) *Runner {
	return &Runner{Program: program, StderrLimit: defaultStderrLimit}
}

// CommandError describes a failed Git process without exposing unbounded stderr.
type CommandError struct {
	Program string
	Args    []string
	Stderr  string
	Err     error
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git command failed: %s", e.Stderr)
	}
	return fmt.Sprintf("git command failed: %v", e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// Run executes Git in directory and returns its standard output.
func (r *Runner) Run(ctx context.Context, directory string, args ...string) ([]byte, error) {
	output, _, err := r.run(ctx, directory, 0, nil, args...)
	return output, err
}

// RunLimited executes Git while retaining at most limit bytes from standard
// output. The boolean result reports whether Git produced additional bytes.
func (r *Runner) RunLimited(ctx context.Context, directory string, limit int, args ...string) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, errors.New("git stdout limit must be positive")
	}
	return r.run(ctx, directory, limit, nil, args...)
}

// RunDiffLimited accepts Git diff's exit code 1, which means differences were
// found rather than that the command failed.
func (r *Runner) RunDiffLimited(ctx context.Context, directory string, limit int, args ...string) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, errors.New("git stdout limit must be positive")
	}
	return r.run(ctx, directory, limit, map[int]bool{1: true}, args...)
}

func (r *Runner) run(ctx context.Context, directory string, stdoutLimit int, acceptedExitCodes map[int]bool, args ...string) ([]byte, bool, error) {
	if r == nil || r.Program == "" {
		return nil, false, errors.New("git runner has no executable")
	}
	if err := validateReadOnlyGitInvocation(args); err != nil {
		return nil, false, err
	}
	commandArgs := make([]string, 0, len(r.prefixArgs)+len(safeGitArguments)+len(args))
	commandArgs = append(commandArgs, r.prefixArgs...)
	commandArgs = append(commandArgs, safeGitArguments...)
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, r.Program, commandArgs...)
	cmd.Dir = directory
	cmd.Env = controlledEnvironment(r.extraEnv)

	var stdout bytes.Buffer
	var limitedStdout *limitedBuffer
	stderr := newLimitedBuffer(r.StderrLimit)
	if stdoutLimit > 0 {
		limitedStdout = newLimitedBuffer(stdoutLimit)
		cmd.Stdout = limitedStdout
	} else {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = stderr

	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok && acceptedExitCodes[exitError.ExitCode()] {
		err = nil
	}
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, false, &CommandError{
			Program: r.Program,
			Args:    append([]string(nil), args...),
			Stderr:  sanitizeDiagnostic(stderr.String()),
			Err:     err,
		}
	}
	if limitedStdout != nil {
		return limitedStdout.Bytes(), limitedStdout.truncated, nil
	}
	return stdout.Bytes(), false, nil
}

func controlledEnvironment(extra []string) []string {
	values := map[string]string{
		"GIT_PAGER":           "",
		"PAGER":               "",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_SYSTEM":   os.DevNull,
		"GIT_OPTIONAL_LOCKS":  "0",
		"GIT_ATTR_NOSYSTEM":   "1",
		"SSH_ASKPASS_REQUIRE": "never",
		"LC_ALL":              "C",
	}
	for _, entry := range extra {
		if key, value, ok := strings.Cut(entry, "="); ok {
			values[key] = value
		}
	}
	// GIT_ALLOW_PROTOCOL is an environment-level allowlist that overrides
	// repository protocol.<name>.allow settings, including custom helpers.
	// An empty value denies every transport on Git 2.31.0 and current Git.
	values["GIT_ALLOW_PROTOCOL"] = ""
	values["GIT_NO_REPLACE_OBJECTS"] = "1"
	values["GIT_PROTOCOL_FROM_USER"] = "0"

	env := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := values[key]; !replaced && !strings.HasPrefix(key, "GIT_") && !strings.HasPrefix(key, "SSH_") && key != "BASH_ENV" && key != "ENV" {
			env = append(env, entry)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

// These command-line overrides win over repository configuration and are also
// inherited by Git subprocesses. They disable every helper class reachable by
// DiffBeacon's read-only command allowlist.
var safeGitArguments = []string{
	"--no-pager",
	"-c", "core.fsmonitor=false",
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "diff.external=",
	"-c", "core.askPass=",
	"-c", "credential.helper=",
	"-c", "protocol.allow=never",
	"-c", "protocol.ext.allow=never",
	"-c", "submodule.recurse=false",
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	if limit <= 0 {
		limit = defaultStderrLimit
	}
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || originalLength > 0
		return originalLength, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(data)
	return originalLength, nil
}

func (b *limitedBuffer) String() string {
	if b.truncated {
		return b.buffer.String() + " [truncated]"
	}
	return b.buffer.String()
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func sanitizeDiagnostic(value string) string {
	return sanitize.Text(strings.TrimSpace(value))
}
