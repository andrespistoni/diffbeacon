package testrepo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type MissingPromisorBlobs struct {
	Repository *Repository
	IndexPath  string
	IndexOID   string
	TreePath   string
	TreeOID    string
}

// NewMissingPromisorBlobs creates a real blob:none clone whose index and HEAD
// each refer to a promised baseline blob that is absent locally.
func NewMissingPromisorBlobs(t testing.TB) MissingPromisorBlobs {
	t.Helper()
	source := New(t)
	const indexPath = "a-index-missing.txt"
	const treePath = "b-tree-missing.txt"
	source.Write(indexPath, "promised index baseline\n")
	source.Write(treePath, "promised tree baseline\n")
	source.CommitAll("promisor baseline")
	source.Git("config", "uploadpack.allowFilter", "true")
	source.Git("config", "uploadpack.allowAnySHA1InWant", "true")
	indexOID := source.Git("rev-parse", "HEAD:"+indexPath)
	treeOID := source.Git("rev-parse", "HEAD:"+treePath)

	root := filepath.Join(t.TempDir(), "partial")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create partial-clone HOME: %v", err)
	}
	command := exec.Command("git", "clone", "--quiet", "--filter=blob:none", "--no-checkout", "--no-local", "file://"+source.Path, root)
	command.Env = environmentWithOverrides(isolatedEnvironment(home), map[string]string{"GIT_ALLOW_PROTOCOL": "file"})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create partial clone: %v\n%s", err, output)
	}
	repository := &Repository{Path: root, Home: home, t: t}
	repository.gitWithEnvironment(map[string]string{"GIT_ALLOW_PROTOCOL": ""}, "read-tree", "HEAD")
	repository.Write(indexPath, "working tree differs from promised index\n")
	repository.Write(treePath, "staged content differs from promised tree\n")
	stagedOID := repository.gitWithEnvironment(map[string]string{"GIT_ALLOW_PROTOCOL": ""}, "hash-object", "-w", "--", treePath)
	repository.gitWithEnvironment(map[string]string{"GIT_ALLOW_PROTOCOL": ""}, "update-index", "--add", "--cacheinfo", "100644,"+stagedOID+","+treePath)

	missing := repository.gitWithEnvironment(map[string]string{"GIT_ALLOW_PROTOCOL": ""}, "rev-list", "--objects", "--missing=print", "--no-object-names", "HEAD")
	for _, oid := range []string{indexOID, treeOID} {
		if !strings.Contains("\n"+missing+"\n", "\n?"+oid+"\n") {
			t.Fatalf("promisor fixture object %s is not reported missing:\n%s", oid, missing)
		}
	}
	return MissingPromisorBlobs{Repository: repository, IndexPath: indexPath, IndexOID: indexOID, TreePath: treePath, TreeOID: treeOID}
}

func (r *Repository) gitWithEnvironment(overrides map[string]string, args ...string) string {
	r.t.Helper()
	command := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	command.Dir = r.Path
	command.Env = environmentWithOverrides(isolatedEnvironment(r.Home), overrides)
	output, err := command.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (r *Repository) Remove(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Path, filepath.FromSlash(path))); err != nil {
		r.t.Fatalf("remove %q: %v", path, err)
	}
}

func (r *Repository) Rename(oldPath, newPath string) {
	r.t.Helper()
	oldFullPath := filepath.Join(r.Path, filepath.FromSlash(oldPath))
	newFullPath := filepath.Join(r.Path, filepath.FromSlash(newPath))
	if err := os.MkdirAll(filepath.Dir(newFullPath), 0o755); err != nil {
		r.t.Fatalf("create rename destination for %q: %v", newPath, err)
	}
	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		r.t.Fatalf("rename %q to %q: %v", oldPath, newPath, err)
	}
}

func (r *Repository) ReplaceWithSymlink(path, target string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Path, filepath.FromSlash(path))
	if err := os.Remove(fullPath); err != nil {
		r.t.Fatalf("remove %q before symlink: %v", path, err)
	}
	if err := os.Symlink(target, fullPath); err != nil {
		r.t.Fatalf("create symlink %q: %v", path, err)
	}
}

func NewConflict(t testing.TB, path string) *Repository {
	t.Helper()
	repository := New(t)
	repository.Write(path, "base\n")
	repository.CommitAll("base")

	repository.Git("switch", "--quiet", "-c", "other")
	repository.Write(path, "other\n")
	repository.CommitAll("other change")

	repository.Git("switch", "--quiet", "main")
	repository.Write(path, "main\n")
	repository.CommitAll("main change")
	if err := repository.GitError("merge", "--no-edit", "other"); err == nil {
		t.Fatal("git merge unexpectedly succeeded; conflict fixture is invalid")
	}
	return repository
}

func NewSubmoduleChange(t testing.TB) (*Repository, string) {
	t.Helper()
	source := New(t)
	source.Write("dependency.txt", "base\n")
	source.CommitAll("dependency base")

	superproject := New(t)
	path := "modules/dependency"
	superproject.Git("-c", "protocol.file.allow=always", "submodule", "add", "--quiet", source.Path, path)
	superproject.CommitAll("add submodule")

	checkout := superproject.At(path)
	checkout.Write("dependency.txt", "modified in submodule\n")
	return superproject, path
}

// NewTripleContent creates one path whose HEAD, index and working-tree content
// are all different.
func NewTripleContent(t testing.TB, path string) *Repository {
	t.Helper()
	repository := New(t)
	repository.Write(path, "head\n")
	repository.CommitAll("base")
	repository.Write(path, "index\n")
	repository.Git("add", "--", path)
	repository.Write(path, "working tree\n")
	return repository
}

// WorkingTreeSnapshot captures paths, file modes, regular-file bytes, and
// symlink targets below the worktree while excluding Git's administrative data.
// Its deterministic representation is suitable for proving that an index-only
// operation did not modify the working tree, including hostile file names.
func (r *Repository) WorkingTreeSnapshot() string {
	r.t.Helper()
	var records []string
	err := filepath.WalkDir(r.Path, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(r.Path, path)
		if err != nil {
			return err
		}
		if relative == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == "." {
			return nil
		}
		name := filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			records = append(records, fmt.Sprintf("L %q %q", name, target))
		case entry.IsDir():
			records = append(records, fmt.Sprintf("D %q %04o", name, info.Mode().Perm()))
		case info.Mode().IsRegular():
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			records = append(records, fmt.Sprintf("F %q %04o %x", name, info.Mode().Perm(), content))
		default:
			records = append(records, fmt.Sprintf("O %q %s", name, info.Mode()))
		}
		return nil
	})
	if err != nil {
		r.t.Fatalf("snapshot working tree: %v", err)
	}
	sort.Strings(records)
	return strings.Join(records, "\n")
}

func (r *Repository) GitDir() string {
	r.t.Helper()
	return r.Git("rev-parse", "--absolute-git-dir")
}
