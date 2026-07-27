package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type Repository struct {
	Path string
	Home string
	t    testing.TB
}

func New(t testing.TB) *Repository {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("Git is required for integration tests: %v", err)
	}
	root := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create isolated HOME: %v", err)
	}
	repository := &Repository{Path: root, Home: home, t: t}
	repository.Git("-c", "init.defaultBranch=main", "init", "--quiet")
	repository.Git("config", "user.name", "DiffBeacon Tests")
	repository.Git("config", "user.email", "diffbeacon@example.invalid")
	return repository
}

func (r *Repository) Git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = r.Path
	cmd.Env = isolatedEnvironment(r.Home)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (r *Repository) GitError(args ...string) error {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = r.Path
	cmd.Env = isolatedEnvironment(r.Home)
	return cmd.Run()
}

func (r *Repository) Write(path, content string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Path, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		r.t.Fatalf("create parent for %q: %v", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		r.t.Fatalf("write %q: %v", path, err)
	}
}

func (r *Repository) CommitAll(message string) {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "--quiet", "-m", message)
}

func (r *Repository) At(path string) *Repository {
	return &Repository{
		Path: filepath.Join(r.Path, filepath.FromSlash(path)),
		Home: r.Home,
		t:    r.t,
	}
}

func isolatedEnvironment(home string) []string {
	overrides := map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     filepath.Join(home, ".config"),
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		"LC_ALL":              "C",
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := overrides[key]; !replaced {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}
