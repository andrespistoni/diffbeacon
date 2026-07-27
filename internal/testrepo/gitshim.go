package testrepo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	gitShimEnabledEnv = "DIFFBEACON_TEST_GIT_SHIM"
	gitShimRealGitEnv = "DIFFBEACON_TEST_REAL_GIT"
	gitShimLogEnv     = "DIFFBEACON_TEST_PROCESS_LOG"
)

// ProcessRecord is one process intercepted through the E2E PATH. Arguments are
// retained as a JSON array so hostile paths cannot be split or reinterpreted.
type ProcessRecord struct {
	Executable string   `json:"executable"`
	Directory  string   `json:"directory"`
	Arguments  []string `json:"arguments"`
}

// GitShim provides an instrumented PATH containing Git plus sentinels for
// shells, network clients, and destructive standalone utilities.
type GitShim struct {
	Directory string
	LogPath   string
	RealGit   string
}

func NewGitShim(t testing.TB) *GitShim {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("Git is required for E2E tests: %v", err)
	}
	realGit, err = filepath.Abs(realGit)
	if err != nil {
		t.Fatalf("resolve Git executable: %v", err)
	}
	directory := t.TempDir()
	shim := &GitShim{Directory: directory, LogPath: filepath.Join(directory, "processes.jsonl"), RealGit: realGit}
	helper := filepath.Join(directory, "gitshim-helper")
	source := filepath.Join(directory, "gitshim-main.go")
	if err := os.WriteFile(source, []byte(standaloneGitShimSource), 0o600); err != nil {
		t.Fatalf("write standalone Git shim: %v", err)
	}
	build := exec.Command("go", "build", "-o", helper, source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone Git shim: %v\n%s", err, output)
	}
	for _, name := range []string{
		"git",
		"git-remote-probe",
		"sh", "bash", "dash", "zsh", "ksh", "fish", "pwsh", "powershell", "cmd", "cmd.exe",
		"rm", "unlink", "rmdir",
		"curl", "wget", "ssh", "scp", "nc", "telnet",
	} {
		target := filepath.Join(directory, name)
		if err := os.Symlink(helper, target); err != nil {
			content, readErr := os.ReadFile(helper)
			if readErr != nil {
				t.Fatalf("read shim executable: %v", readErr)
			}
			if writeErr := os.WriteFile(target, content, 0o755); writeErr != nil {
				t.Fatalf("copy shim executable: %v", writeErr)
			}
		}
	}
	return shim
}

const standaloneGitShimSource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type processRecord struct {
	Executable string
	Directory string
	Arguments []string
}

func main() {
	executable := filepath.Base(os.Args[0])
	directory, _ := os.Getwd()
	record := processRecord{Executable: executable, Directory: directory, Arguments: append([]string(nil), os.Args[1:]...)}
	encoded, err := json.Marshal(record)
	if err == nil {
		encoded = append(encoded, '\n')
		var file *os.File
		file, err = os.OpenFile(os.Getenv("DIFFBEACON_TEST_PROCESS_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_, err = file.Write(encoded)
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
	if executable != "git" {
		fmt.Fprintf(os.Stderr, "prohibited executable invoked: %s\n", executable)
		os.Exit(126)
	}
	realGit := os.Getenv("DIFFBEACON_TEST_REAL_GIT")
	if realGit == "" {
		fmt.Fprintln(os.Stderr, "real Git path is empty")
		os.Exit(125)
	}
	command := exec.Command(realGit, os.Args[1:]...)
	command.Dir = directory
	command.Env = os.Environ()
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(125)
	}
}
`

// Environment returns a process environment with a shim-only PATH and isolated
// Git configuration. The delegated real Git uses its absolute path.
func (s *GitShim) Environment(home string) []string {
	overrides := map[string]string{
		"PATH":                s.Directory,
		"HOME":                home,
		"XDG_CONFIG_HOME":     filepath.Join(home, ".config"),
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		gitShimEnabledEnv:     "1",
		gitShimRealGitEnv:     s.RealGit,
		gitShimLogEnv:         s.LogPath,
	}
	return environmentWithOverrides(os.Environ(), overrides)
}

func environmentWithOverrides(base []string, overrides map[string]string) []string {
	environment := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func (s *GitShim) Records(t testing.TB) []ProcessRecord {
	t.Helper()
	file, err := os.Open(s.LogPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("open process log: %v", err)
	}
	defer file.Close()
	var records []ProcessRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record ProcessRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode process record %q: %v", scanner.Text(), err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan process log: %v", err)
	}
	return records
}

// RunGitShim is called from an E2E package TestMain before flag parsing. It
// records the invocation, delegates only the executable named git, and rejects
// every sentinel executable.
func RunGitShim() (handled bool, exitCode int) {
	if os.Getenv(gitShimEnabledEnv) != "1" {
		return false, 0
	}
	executable := filepath.Base(os.Args[0])
	directory, _ := os.Getwd()
	record := ProcessRecord{Executable: executable, Directory: directory, Arguments: append([]string(nil), os.Args[1:]...)}
	if err := appendProcessRecord(os.Getenv(gitShimLogEnv), record); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true, 125
	}
	if executable != "git" {
		fmt.Fprintf(os.Stderr, "prohibited executable invoked: %s\n", executable)
		return true, 126
	}
	realGit := os.Getenv(gitShimRealGitEnv)
	if realGit == "" {
		fmt.Fprintln(os.Stderr, "real Git path is empty")
		return true, 125
	}
	command := exec.Command(realGit, os.Args[1:]...)
	command.Dir = directory
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{gitShimEnabledEnv: "", gitShimRealGitEnv: "", gitShimLogEnv: ""})
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return true, exitError.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return true, 125
	}
	return true, 0
}

func appendProcessRecord(path string, record ProcessRecord) error {
	if path == "" {
		return fmt.Errorf("process log path is empty")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(encoded)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
