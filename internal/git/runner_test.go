package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunnerKeepsArgumentsSeparateAndDoesNotUseShell(t *testing.T) {
	runner := helperRunner(t)
	marker := t.TempDir() + "/shell-was-used"
	args := []string{"--no-optional-locks", "--literal-pathspecs", "ls-files", "--stage", "-z", "--", "$(touch " + marker + ") path with spaces\n"}

	output, err := runner.Run(context.Background(), t.TempDir(), args...)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got []string
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("decode helper output: %v; output = %q", err, output)
	}
	if fmt.Sprint(got) != fmt.Sprint(args) {
		t.Fatalf("arguments = %#v, want %#v", got, args)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shell marker exists or stat failed unexpectedly: %v", err)
	}
}

func TestRunnerBoundsAndSanitizesStderr(t *testing.T) {
	runner := helperRunner(t)
	runner.StderrLimit = 32
	runner.extraEnv = append(runner.extraEnv, "DIFFBEACON_TEST_HELPER_MODE=stderr")

	_, err := runner.Run(context.Background(), t.TempDir(), "rev-parse", "--is-bare-repository")
	var commandError *CommandError
	if !errors.As(err, &commandError) {
		t.Fatalf("Run() error = %T %v, want *CommandError", err, err)
	}
	if len(commandError.Stderr) > 64 {
		t.Fatalf("stderr length = %d, want bounded value", len(commandError.Stderr))
	}
	if strings.ContainsRune(commandError.Stderr, '\x1b') {
		t.Fatalf("stderr contains an escape character: %q", commandError.Stderr)
	}
	if !strings.Contains(commandError.Stderr, "[truncated]") {
		t.Fatalf("stderr = %q, want truncation marker", commandError.Stderr)
	}
}

func TestRunnerDropsDangerousGitEnvironmentAndForcesSafePolicy(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "/tmp/hostile-helper")
	t.Setenv("GIT_EXTERNAL_DIFF", "/tmp/hostile-diff")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file:http:probe")
	t.Setenv("GIT_PROTOCOL_FROM_USER", "1")
	t.Setenv("SSH_ASKPASS", "/tmp/hostile-askpass")
	runner := helperRunner(t)
	runner.extraEnv = append(runner.extraEnv,
		"DIFFBEACON_TEST_HELPER_MODE=env",
		"GIT_ALLOW_PROTOCOL=file:http:probe",
		"GIT_NO_REPLACE_OBJECTS=0",
		"GIT_PROTOCOL_FROM_USER=1",
	)
	output, err := runner.Run(context.Background(), t.TempDir(), "rev-parse", "--is-bare-repository")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_EXTERNAL_DIFF", "SSH_ASKPASS"} {
		if got[key] != "" {
			t.Fatalf("%s survived as %q", key, got[key])
		}
	}
	if got["GIT_CONFIG_NOSYSTEM"] != "1" || got["GIT_TERMINAL_PROMPT"] != "0" || got["GIT_PAGER"] != "" || got["GIT_ALLOW_PROTOCOL"] != "" || got["GIT_NO_REPLACE_OBJECTS"] != "1" || got["GIT_PROTOCOL_FROM_USER"] != "0" {
		t.Fatalf("safe environment = %#v", got)
	}
}

func TestRunnerHonorsContextCancellation(t *testing.T) {
	runner := helperRunner(t)
	runner.extraEnv = append(runner.extraEnv, "DIFFBEACON_TEST_HELPER_MODE=wait")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.Run(ctx, t.TempDir(), "rev-parse", "--is-bare-repository")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context deadline exceeded", err)
	}
}

func TestRunnerBoundsStdout(t *testing.T) {
	runner := helperRunner(t)
	output, truncated, err := runner.RunLimited(context.Background(), t.TempDir(), 4, "rev-parse", "--is-bare-repository")
	if err != nil {
		t.Fatalf("RunLimited() error = %v", err)
	}
	if string(output) != "[\"re" || !truncated {
		t.Fatalf("RunLimited() = %q, %v; want first four bytes and truncation", output, truncated)
	}
}

func TestRunnerRejectsMutationBeforeStartingProcess(t *testing.T) {
	runner := helperRunner(t)
	marker := t.TempDir() + "/process-started"
	runner.extraEnv = append(runner.extraEnv, "DIFFBEACON_TEST_HELPER_MARKER="+marker)
	_, err := runner.Run(context.Background(), t.TempDir(), "add", "--", "file.txt")
	if !errors.Is(err, ErrGitCommandNotReadOnly) {
		t.Fatalf("Run() error = %v, want ErrGitCommandNotReadOnly", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected process started: %v", statErr)
	}
}

func TestGitProcessHelper(t *testing.T) {
	if os.Getenv("DIFFBEACON_GIT_HELPER") != "1" {
		return
	}

	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(3)
	}
	args := os.Args[separator+1:]
	if len(args) < len(safeGitArguments) || fmt.Sprint(args[:len(safeGitArguments)]) != fmt.Sprint(safeGitArguments) {
		os.Exit(6)
	}
	args = args[len(safeGitArguments):]
	if marker := os.Getenv("DIFFBEACON_TEST_HELPER_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started"), 0o600)
	}
	if args[0] == "--version" {
		_, _ = os.Stdout.WriteString(os.Getenv("DIFFBEACON_TEST_GIT_VERSION") + "\n")
		os.Exit(0)
	}
	if os.Getenv("DIFFBEACON_TEST_HELPER_MODE") == "stderr" {
		_, _ = os.Stderr.WriteString("bad\x1b[31m" + strings.Repeat("x", 4096))
		os.Exit(7)
	}
	if os.Getenv("DIFFBEACON_TEST_HELPER_MODE") == "wait" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	if os.Getenv("DIFFBEACON_TEST_HELPER_MODE") == "env" {
		keys := []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_EXTERNAL_DIFF", "SSH_ASKPASS", "GIT_CONFIG_NOSYSTEM", "GIT_TERMINAL_PROMPT", "GIT_PAGER", "GIT_ALLOW_PROTOCOL", "GIT_NO_REPLACE_OBJECTS", "GIT_PROTOCOL_FROM_USER"}
		values := make(map[string]string, len(keys))
		for _, key := range keys {
			values[key] = os.Getenv(key)
		}
		_ = json.NewEncoder(os.Stdout).Encode(values)
		os.Exit(0)
	}
	if containsTestArgument(args, "status") && os.Getenv("DIFFBEACON_TEST_SLOW_STATUS") == "1" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	if err := json.NewEncoder(os.Stdout).Encode(args); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func containsTestArgument(arguments []string, wanted string) bool {
	for _, argument := range arguments {
		if argument == wanted {
			return true
		}
	}
	return false
}

func helperRunner(t *testing.T) *Runner {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	runner := NewRunner(executable)
	runner.prefixArgs = []string{"-test.run=TestGitProcessHelper", "--"}
	runner.extraEnv = []string{"DIFFBEACON_GIT_HELPER=1"}
	return runner
}
