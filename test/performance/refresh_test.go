package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const target = 500 * time.Millisecond

type latencyReport struct {
	Timestamp          string  `json:"timestamp_utc"`
	OS                 string  `json:"os"`
	Architecture       string  `json:"architecture"`
	LogicalCPUs        int     `json:"logical_cpus"`
	GoVersion          string  `json:"go_version"`
	GitVersion         string  `json:"git_version"`
	Hardware           string  `json:"hardware"`
	Filesystem         string  `json:"filesystem"`
	FixtureFiles       int     `json:"fixture_files"`
	FixtureChanged     int     `json:"fixture_changed_files"`
	FixtureBytes       int     `json:"fixture_worktree_bytes"`
	Samples            int     `json:"samples_per_metric"`
	PercentileMethod   string  `json:"percentile_method"`
	TargetMilliseconds float64 `json:"target_milliseconds"`
	Startup            metric  `json:"startup_to_first_useful_view"`
	Refresh            metric  `json:"save_to_updated_view"`
}

type metric struct {
	P50Milliseconds     float64 `json:"p50_milliseconds"`
	P95Milliseconds     float64 `json:"p95_milliseconds"`
	MaximumMilliseconds float64 `json:"maximum_milliseconds"`
	TargetMet           bool    `json:"target_met"`
}

func TestRefreshLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("performance measurement is disabled by -short")
	}
	samples := sampleCount(t)
	root, home, fixtureBytes := createFixture(t)
	binary := buildBinary(t)

	startup := make([]time.Duration, 0, samples)
	for index := 0; index < samples; index++ {
		marker := fmt.Sprintf("startup-marker-%03d", index)
		writeFocus(t, root, marker)
		started := time.Now()
		session := startSession(t, binary, root, home)
		session.output.waitFor(t, marker, 0)
		startup = append(startup, time.Since(started))
		session.stop(t)
	}

	writeFocus(t, root, "refresh-baseline")
	session := startSession(t, binary, root, home)
	session.output.waitFor(t, "refresh-baseline", 0)
	refresh := make([]time.Duration, 0, samples)
	for index := 0; index < samples; index++ {
		marker := fmt.Sprintf("refresh-marker-%03d", index)
		offset := session.output.length()
		writeFocus(t, root, marker)
		started := time.Now()
		session.output.waitFor(t, marker, offset)
		refresh = append(refresh, time.Since(started))
	}
	session.stop(t)

	report := latencyReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339), OS: runtime.GOOS, Architecture: runtime.GOARCH,
		LogicalCPUs: runtime.NumCPU(), GoVersion: runtime.Version(), GitVersion: commandOutput(t, "git", "--version"),
		Hardware:     hardwareDescription(),
		Filesystem:   nonEmpty(os.Getenv("DIFFBEACON_PERF_FILESYSTEM"), "not reported"),
		FixtureFiles: 200, FixtureChanged: 25, FixtureBytes: fixtureBytes, Samples: samples,
		PercentileMethod: "nearest-rank (ceil(p*n), 1-based)", TargetMilliseconds: milliseconds(target),
		Startup: summarize(startup), Refresh: summarize(refresh),
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PERF_RESULT %s", encoded)
	if path := os.Getenv("DIFFBEACON_PERF_REPORT"); path != "" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
			t.Fatalf("write performance report: %v", err)
		}
	}
	if !report.Startup.TargetMet || !report.Refresh.TargetMet {
		t.Logf("RELEASE_RISK latency target missed: startup p95=%.2fms refresh p95=%.2fms target<%.0fms", report.Startup.P95Milliseconds, report.Refresh.P95Milliseconds, report.TargetMilliseconds)
	}
	if os.Getenv("DIFFBEACON_PERF_ENFORCE") == "1" && (!report.Startup.TargetMet || !report.Refresh.TargetMet) {
		t.Fatal("latency target missed with DIFFBEACON_PERF_ENFORCE=1")
	}
}

func createFixture(t *testing.T) (string, string, int) {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	runGit(t, root, home, "init", "--quiet", "-b", "main")
	runGit(t, root, home, "config", "user.name", "DiffBeacon Performance")
	runGit(t, root, home, "config", "user.email", "performance@example.invalid")
	total := 0
	for index := 0; index < 200; index++ {
		var content strings.Builder
		for line := 0; line < 40; line++ {
			fmt.Fprintf(&content, "file=%03d line=%02d deterministic fixture content\n", index, line)
		}
		value := content.String()
		total += len(value)
		name := filepath.Join(root, fmt.Sprintf("file-%03d.txt", index))
		if err := os.WriteFile(name, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, home, "add", "-A")
	runGit(t, root, home, "commit", "--quiet", "-m", "performance fixture")
	for index := 0; index < 25; index++ {
		name := filepath.Join(root, fmt.Sprintf("file-%03d.txt", index))
		value := fmt.Sprintf("initial-change-%03d\n", index) + strings.Repeat("stable context line\n", 39)
		if err := os.WriteFile(name, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, home, total
}

func writeFocus(t *testing.T, root, marker string) {
	t.Helper()
	value := marker + "\n" + strings.Repeat("stable context line\n", 39)
	if err := os.WriteFile(filepath.Join(root, "file-000.txt"), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diffbeacon")
	command := exec.Command("go", "build", "-trimpath", "-o", path, "./cmd/diffbeacon")
	command.Dir = repositoryRoot()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build benchmark binary: %v\n%s", err, output)
	}
	return path
}

type session struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	output  *observedOutput
	done    chan error
}

func startSession(t *testing.T, binary, root, home string) *session {
	t.Helper()
	command := exec.Command(binary, root)
	command.Env = isolatedEnvironment(home)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output := newObservedOutput()
	command.Stdout = output
	var stderr bytes.Buffer
	command.Stderr = &stderr
	started := &session{command: command, stdin: stdin, output: output, done: make(chan error, 1)}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { started.done <- command.Wait() }()
	t.Cleanup(func() {
		if command.ProcessState == nil || !command.ProcessState.Exited() {
			_ = command.Process.Kill()
		}
	})
	return started
}

func (s *session) stop(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(s.stdin, "q"); err != nil {
		t.Fatalf("stop benchmark session: %v", err)
	}
	select {
	case err := <-s.done:
		_ = s.stdin.Close()
		if err != nil {
			t.Fatalf("benchmark session exited: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = s.command.Process.Kill()
		t.Fatal("benchmark session did not stop")
	}
}

type observedOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	notify chan struct{}
}

func newObservedOutput() *observedOutput {
	return &observedOutput{notify: make(chan struct{}, 1)}
}

func (o *observedOutput) Write(value []byte) (int, error) {
	o.mu.Lock()
	n, err := o.buffer.Write(value)
	o.mu.Unlock()
	select {
	case o.notify <- struct{}{}:
	default:
	}
	return n, err
}

func (o *observedOutput) length() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.Len()
}

func (o *observedOutput) containsAfter(value string, offset int) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	data := o.buffer.Bytes()
	if offset > len(data) {
		offset = len(data)
	}
	return bytes.Contains(data[offset:], []byte(value))
}

func (o *observedOutput) waitFor(t *testing.T, value string, offset int) {
	t.Helper()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for !o.containsAfter(value, offset) {
		select {
		case <-o.notify:
		case <-timer.C:
			t.Fatalf("timed out waiting for rendered marker %q", value)
		}
	}
}

func summarize(values []time.Duration) metric {
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p50 := nearestRank(sorted, 0.50)
	p95 := nearestRank(sorted, 0.95)
	return metric{P50Milliseconds: milliseconds(p50), P95Milliseconds: milliseconds(p95), MaximumMilliseconds: milliseconds(sorted[len(sorted)-1]), TargetMet: p95 < target}
}

func nearestRank(sorted []time.Duration, percentile float64) time.Duration {
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}

func milliseconds(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}

func sampleCount(t *testing.T) int {
	t.Helper()
	value := nonEmpty(os.Getenv("DIFFBEACON_PERF_SAMPLES"), "20")
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 20 {
		t.Fatalf("DIFFBEACON_PERF_SAMPLES = %q, want integer >= 20", value)
	}
	return parsed
}

func runGit(t *testing.T, root, home string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = isolatedEnvironment(home)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func isolatedEnvironment(home string) []string {
	values := map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0", "LC_ALL": "C"}
	result := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := values[key]; !replaced {
				result = append(result, entry)
			}
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func commandOutput(t *testing.T, name string, args ...string) string {
	t.Helper()
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return strings.TrimSpace(string(output))
}

func hardwareDescription() string {
	if value := os.Getenv("DIFFBEACON_PERF_HARDWARE"); value != "" {
		return value
	}
	if runtime.GOOS == "linux" {
		if content, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(content), "\n") {
				key, value, ok := strings.Cut(line, ":")
				if ok && strings.TrimSpace(key) == "model name" {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	return "not reported"
}

func repositoryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
