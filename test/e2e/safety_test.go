package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"diffbeacon/internal/testrepo"
)

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
	buildDir   string
)

func TestMain(m *testing.M) {
	if handled, code := testrepo.RunGitShim(); handled {
		os.Exit(code)
	}
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func TestDiffBeaconExternalProcess(t *testing.T) {
	handled, err := testrepo.RunExternalProcess()
	if handled && err != nil {
		t.Fatal(err)
	}
}

func TestBinaryAllInteractionsAreReadOnly(t *testing.T) {
	fixture := testrepo.New(t)
	fixture.Write("review.txt", "base\n")
	fixture.Write("staged.txt", "base\n")
	fixture.CommitAll("base")
	fixture.Write("review.txt", "working tree must survive\n")
	fixture.Write("staged.txt", "externally staged\n")
	fixture.ExternalGit("add", "--", "staged.txt")
	fixture.Write("untracked.txt", "untracked\n")
	before := fixture.Snapshot()
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, "review.txt")
	waitForPathDetailLoad(t, shim, "review.txt")
	statusCalls := countGitCommand(shim.Records(t), "status")
	for _, input := range []string{"j", "k", "h", "l", "\t", "\r", "\x1b", "[", "]", "v", "f", "1", "2", "3", "4", "e", "?", "?", "s", "u", "S", "U", "\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D"} {
		session.send(t, input)
	}
	session.send(t, "r")
	waitFor(t, "manual read-only refresh", func() bool { return countGitCommand(shim.Records(t), "status") > statusCalls })
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	session.quit(t)
	assertRepositorySnapshotEqual(t, before, fixture.Snapshot())
	assertSafeProcessRecords(t, shim.Records(t))
}

func TestBinaryPromisorErrorsStayLocalNonFatalAndImmutable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start promisor network observer: %v", err)
	}
	defer listener.Close()
	attempts := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			attempts <- connection
		}
	}()

	promisor := testrepo.NewMissingPromisorBlobs(t)
	fixture := promisor.Repository
	fixture.ExternalGit("config", "remote.origin.url", "http://"+listener.Addr().String()+"/instrumented.git")
	fixture.ExternalGit("config", "protocol.allow", "always")
	fixture.ExternalGit("config", "protocol.http.allow", "always")
	before := fixture.Snapshot()
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, promisor.IndexPath)
	waitForPathDetailLoad(t, shim, promisor.IndexPath)
	waitForOutputContains(t, session, "content load failed")

	session.send(t, "j")
	waitForPathDetailLoad(t, shim, promisor.TreePath)
	statusCalls := countGitCommand(shim.Records(t), "status")
	session.send(t, "r")
	waitFor(t, "refresh after local promisor degradation", func() bool { return countGitCommand(shim.Records(t), "status") > statusCalls })
	session.quit(t)

	select {
	case connection := <-attempts:
		_ = connection.Close()
		t.Fatal("DiffBeacon connected to the instrumented promisor remote")
	case <-time.After(200 * time.Millisecond):
	}
	assertRepositorySnapshotEqual(t, before, fixture.Snapshot())
	records := shim.Records(t)
	for _, record := range records {
		if gitCommand(record.Arguments) == "cat-file" && (containsArgument(record.Arguments, promisor.IndexOID) || containsArgument(record.Arguments, promisor.TreeOID)) {
			t.Errorf("missing promisor blob reached cat-file instead of failing before process start: %#v", record.Arguments)
		}
	}
	assertSafeProcessRecords(t, records)
}

func TestBinaryKeepsIndexLockAndRecoversAfterOwnerRemovesIt(t *testing.T) {
	fixture := testrepo.New(t)
	fixture.Write("locked.txt", "base\n")
	fixture.CommitAll("base")
	fixture.Write("locked.txt", "changed\n")
	lockPath := filepath.Join(fixture.Path, ".git", "index.lock")
	lockContent := []byte("owned externally")
	if err := os.WriteFile(lockPath, lockContent, 0o600); err != nil {
		t.Fatal(err)
	}
	before := fixture.Snapshot()
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, "locked.txt")
	waitForPathDetailLoad(t, shim, "locked.txt")
	statusCalls := countGitCommand(shim.Records(t), "status")
	session.send(t, "rsuSU")
	waitFor(t, "refresh while index lock exists", func() bool { return countGitCommand(shim.Records(t), "status") > statusCalls })
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	if got := readFile(t, lockPath); !bytes.Equal(got, lockContent) {
		t.Fatalf("index.lock changed: %q", got)
	}
	session.quit(t)
	assertRepositorySnapshotEqual(t, before, fixture.Snapshot())
	assertSafeProcessRecords(t, shim.Records(t))
}

func TestBinaryDoesNotExecuteRepositoryFSMonitor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable helper fixture is Unix-only")
	}
	fixture := testrepo.New(t)
	fixture.Write("tracked.txt", "base\n")
	fixture.CommitAll("base")
	marker := filepath.Join(t.TempDir(), "fsmonitor-ran")
	helper := e2eMarkerHelper(t, marker)
	fixture.Git("config", "core.fsmonitor", helper)
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, "No changes in all")
	session.quit(t)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("repository fsmonitor executed or marker stat failed: %v", err)
	}
}

func TestBinaryDoesNotExecuteExternalCleanFilterOrChangeRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable helper fixture is Unix-only")
	}
	fixture := testrepo.New(t)
	fixture.Write(".gitattributes", "filtered.txt filter=hostile\n")
	fixture.Write("filtered.txt", "base\n")
	fixture.CommitAll("base")
	marker := filepath.Join(t.TempDir(), "filter-ran")
	fixture.Git("config", "filter.hostile.clean", e2eMarkerHelper(t, marker))
	fixture.Write("filtered.txt", "changed\n")
	before := fixture.Snapshot()
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, "filtered.txt")
	waitForPathDetailLoad(t, shim, "filtered.txt")
	session.send(t, "suSUr")
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	session.quit(t)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean filter executed or marker stat failed: %v", err)
	}
	assertRepositorySnapshotEqual(t, before, fixture.Snapshot())
	assertSafeProcessRecords(t, shim.Records(t))
}

func e2eMarkerHelper(t *testing.T, marker string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper")
	content := "#!/bin/sh\ntouch '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\ncat\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

type binarySession struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *lockedBuffer
	stderr  *lockedBuffer
	done    chan error
}

func startBinary(t *testing.T, fixture *testrepo.Repository, shim *testrepo.GitShim, environment map[string]string) *binarySession {
	t.Helper()
	command := exec.Command(builtBinary(t), fixture.Path)
	command.Env = withEnvironment(shim.Environment(fixture.Home), environment)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	session := &binarySession{command: command, stdin: stdin, stdout: &lockedBuffer{}, stderr: &lockedBuffer{}, done: make(chan error, 1)}
	command.Stdout, command.Stderr = session.stdout, session.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("diffbeacon stdout: %q", session.stdout.String())
			t.Logf("diffbeacon stderr: %q", session.stderr.String())
		}
		if command.ProcessState == nil || !command.ProcessState.Exited() {
			_ = command.Process.Kill()
		}
	})
	go func() { session.done <- command.Wait() }()
	waitForOutputContains(t, session, enterAltScreen)
	return session
}

func (s *binarySession) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := io.WriteString(s.stdin, keys); err != nil {
		t.Fatalf("send %q: %v", keys, err)
	}
}

func (s *binarySession) quit(t *testing.T) {
	t.Helper()
	s.send(t, "q")
	s.wait(t, false)
}

func (s *binarySession) wait(t *testing.T, allowInterrupt bool) {
	t.Helper()
	select {
	case err := <-s.done:
		_ = s.stdin.Close()
		if err != nil && !allowInterrupt {
			t.Fatalf("diffbeacon exited with %v; stderr=%q", err, s.stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = s.command.Process.Kill()
		t.Fatalf("diffbeacon did not exit; stderr=%q", s.stderr.String())
	}
}

func builtBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		buildDir, binaryErr = os.MkdirTemp("", "diffbeacon-e2e-")
		if binaryErr != nil {
			return
		}
		binaryPath = filepath.Join(buildDir, "diffbeacon")
		command := exec.Command("go", "build", "-o", binaryPath, "./cmd/diffbeacon")
		command.Dir = repositoryRoot()
		output, err := command.CombinedOutput()
		if err != nil {
			binaryErr = fmt.Errorf("go build: %w: %s", err, output)
		}
	})
	if binaryErr != nil {
		t.Fatal(binaryErr)
	}
	return binaryPath
}

func repositoryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func waitForOutputContains(t *testing.T, session *binarySession, value string) {
	t.Helper()
	waitFor(t, "output containing "+fmt.Sprintf("%q", value), func() bool {
		return strings.Contains(session.stdout.String(), value)
	})
}

func waitFor(t *testing.T, description string, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

type fileState struct {
	Mode    os.FileMode
	Content string
}

func workingTreeSnapshot(t *testing.T, root string) map[string]fileState {
	t.Helper()
	snapshot := make(map[string]fileState)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".git" {
			return filepath.SkipDir
		}
		if relative == "." || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := fileState{Mode: info.Mode()}
		if info.Mode()&os.ModeSymlink != 0 {
			state.Content, err = os.Readlink(path)
		} else {
			state.Content = string(readFile(t, path))
		}
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = state
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot working tree: %v", err)
	}
	return snapshot
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func withEnvironment(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := overrides[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func containsArgument(arguments []string, value string) bool {
	for _, argument := range arguments {
		if argument == value {
			return true
		}
	}
	return false
}

func countGitCommand(records []testrepo.ProcessRecord, wanted string) int {
	count := 0
	for _, record := range records {
		if gitCommand(record.Arguments) == wanted {
			count++
		}
	}
	return count
}

func gitCommand(arguments []string) string {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--version" {
			return "version"
		}
		if argument == "-c" && index+1 < len(arguments) {
			index++
			continue
		}
		if strings.HasPrefix(argument, "-") {
			continue
		}
		return argument
	}
	return ""
}

func assertSafeProcessRecords(t *testing.T, records []testrepo.ProcessRecord) {
	t.Helper()
	if len(records) == 0 {
		t.Fatal("process shim recorded no Git activity")
	}
	allowed := map[string]bool{
		"version":   true,
		"rev-parse": true, "status": true,
		"ls-files": true, "cat-file": true, "ls-tree": true, "diff": true,
	}
	for _, record := range records {
		if record.Executable != "git" {
			t.Errorf("prohibited process invoked: %#v", record)
			continue
		}
		command := gitCommand(record.Arguments)
		if !allowed[command] {
			t.Errorf("Git command %q is outside the v0.1 local allowlist: %#v", command, record.Arguments)
		}
	}
}

func waitForPathDetailLoad(t *testing.T, shim *testrepo.GitShim, path string) {
	t.Helper()
	waitFor(t, "detail load for "+path, func() bool {
		for _, record := range shim.Records(t) {
			if containsArgument(record.Arguments, path) && (gitCommand(record.Arguments) == "ls-files" || gitCommand(record.Arguments) == "ls-tree") {
				return true
			}
		}
		return false
	})
}

func waitForProcessQuiescence(t *testing.T, shim *testrepo.GitShim, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	lastCount := -1
	stableSince := time.Now()
	for time.Now().Before(deadline) {
		count := len(shim.Records(t))
		if count != lastCount {
			lastCount = count
			stableSince = time.Now()
		} else if time.Since(stableSince) >= duration {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("process activity did not become quiescent")
}

func runWithTimeout(t *testing.T, duration time.Duration, fn func(context.Context)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	fn(ctx)
}

func assertRepositorySnapshotEqual(t *testing.T, before, after testrepo.RepositorySnapshot) {
	t.Helper()
	if before.GitDir != after.GitDir {
		t.Error("Git directory changed")
	}
	if before.Index != after.Index {
		t.Error("Git index changed")
	}
	if before.WorkingTree != after.WorkingTree {
		t.Error("working tree changed")
	}
	if before.Refs != after.Refs {
		t.Error("Git refs changed")
	}
	if before.Config != after.Config {
		t.Error("Git or home configuration changed")
	}
	if before.Objects != after.Objects {
		t.Error("Git objects changed")
	}
	if before.Packs != after.Packs {
		t.Error("Git packs changed")
	}
	if before.Metadata != after.Metadata {
		t.Error("Git metadata changed")
	}
}
