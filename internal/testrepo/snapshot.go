package testrepo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RepositorySnapshot separates every repository surface covered by the
// read-only contract so failures identify what changed.
type RepositorySnapshot struct {
	GitDir      string
	Index       string
	WorkingTree string
	Refs        string
	Config      string
	Objects     string
	Packs       string
	Metadata    string
}

func (r *Repository) Snapshot() RepositorySnapshot {
	r.t.Helper()
	gitDir := r.GitDir()
	return RepositorySnapshot{
		GitDir:      snapshotTree(gitDir, nil),
		Index:       snapshotEntry(filepath.Join(gitDir, "index")),
		WorkingTree: r.WorkingTreeSnapshot(),
		Refs:        snapshotSelected(gitDir, func(name string) bool { return name == "HEAD" || name == "refs" || strings.HasPrefix(name, "refs/") }),
		Config:      snapshotEntry(filepath.Join(gitDir, "config")) + "\nHOME\n" + snapshotTree(r.Home, nil),
		Objects:     snapshotTree(filepath.Join(gitDir, "objects"), nil),
		Packs:       snapshotTree(filepath.Join(gitDir, "objects", "pack"), nil),
		Metadata: snapshotSelected(gitDir, func(name string) bool {
			return name != "index" && name != "HEAD" && name != "config" && name != "objects" && !strings.HasPrefix(name, "objects/") && name != "refs" && !strings.HasPrefix(name, "refs/")
		}),
	}
}

func snapshotSelected(root string, include func(string) bool) string {
	return snapshotTree(root, func(name string, entry os.DirEntry) bool {
		if include(name) {
			return true
		}
		if entry.IsDir() {
			prefix := name + "/"
			for _, candidate := range []string{"refs/"} {
				if strings.HasPrefix(candidate, prefix) {
					return true
				}
			}
		}
		return false
	})
}

func snapshotTree(root string, include func(string, os.DirEntry) bool) string {
	var records []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if include != nil && !include(name, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		records = append(records, name+" "+snapshotEntry(path))
		return nil
	})
	if err != nil {
		return "ERROR " + err.Error()
	}
	sort.Strings(records)
	return strings.Join(records, "\n")
}

func snapshotEntry(path string) string {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil {
		return "error:" + err.Error()
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "error:" + err.Error()
		}
		return fmt.Sprintf("link:%04o:%s", info.Mode().Perm(), target)
	}
	if info.IsDir() {
		return fmt.Sprintf("dir:%04o", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "error:" + err.Error()
	}
	return fmt.Sprintf("file:%04o:%x", info.Mode().Perm(), sha256.Sum256(content))
}
