package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(want) != got+"\n" {
		t.Fatalf("golden %s mismatch\n--- want ---\n%s--- got ---\n%s\n", path, want, got)
	}
}
