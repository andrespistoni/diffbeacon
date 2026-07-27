package git

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestStatusParserFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/status/all-records.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	data = []byte(strings.ReplaceAll(strings.TrimSuffix(string(data), "\n"), "<NUL>", "\x00"))

	snapshot, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	if got, want := snapshot.Revision, (Revision{HeadOID: "0123456789abcdef", Branch: "main"}); got != want {
		t.Fatalf("Revision = %#v, want %#v", got, want)
	}

	want := []Change{
		{Path: "added.txt", Scope: ScopeStaged, Status: StatusAdded},
		{Path: "double scope.txt", Scope: ScopeStaged, Status: StatusModified},
		{Path: "double scope.txt", Scope: ScopeUnstaged, Status: StatusDeleted},
		{Path: "typed", Scope: ScopeUnstaged, Status: StatusTypeChanged},
		{Path: "new name.txt", OldPath: "old name.txt", Scope: ScopeStaged, Status: StatusRenamed},
		{Path: "copy.txt", OldPath: "source.txt", Scope: ScopeStaged, Status: StatusCopied},
		{Path: "conflict.txt", Scope: ScopeUnstaged, Status: StatusUnmerged, Conflict: ConflictBothModified},
		{Path: "new file.txt", Scope: ScopeUntracked, Status: StatusUntracked},
	}
	if fmt.Sprintf("%#v", snapshot.Changes) != fmt.Sprintf("%#v", want) {
		t.Fatalf("Changes = %#v, want %#v", snapshot.Changes, want)
	}
}

func TestStatusParserCoversMinimumStates(t *testing.T) {
	records := []string{
		trackedRecord("A.", "added"),
		trackedRecord(".M", "modified"),
		trackedRecord("D.", "deleted"),
		trackedRecord(".T", "type"),
		"? untracked",
	}
	data := []byte(strings.Join(records, "\x00") + "\x00")
	snapshot, err := ParseStatus(data)
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	want := []ChangeStatus{StatusAdded, StatusModified, StatusDeleted, StatusTypeChanged, StatusUntracked}
	for index, status := range want {
		if snapshot.Changes[index].Status != status {
			t.Errorf("change %d status = %v, want %v", index, snapshot.Changes[index].Status, status)
		}
	}
}

func TestStatusParserPreservesHostilePathsAndDoubleScope(t *testing.T) {
	paths := []string{
		"path with spaces.txt",
		"unicodé-文件.txt",
		"line\nbreak.txt",
		"$special;[chars]*.txt",
		"-leading-option.txt",
	}
	var records []string
	for _, path := range paths {
		records = append(records, trackedRecord("MM", path))
	}
	snapshot, err := ParseStatus([]byte(strings.Join(records, "\x00") + "\x00"))
	if err != nil {
		t.Fatalf("ParseStatus() error = %v", err)
	}
	if len(snapshot.Changes) != len(paths)*2 {
		t.Fatalf("len(Changes) = %d, want %d", len(snapshot.Changes), len(paths)*2)
	}
	for index, path := range paths {
		staged := snapshot.Changes[index*2]
		unstaged := snapshot.Changes[index*2+1]
		if staged.Identity() != (ChangeIdentity{Path: path, Scope: ScopeStaged}) {
			t.Errorf("staged identity = %#v", staged.Identity())
		}
		if unstaged.Identity() != (ChangeIdentity{Path: path, Scope: ScopeUnstaged}) {
			t.Errorf("unstaged identity = %#v", unstaged.Identity())
		}
	}
}

func TestStatusParserRejectsMalformedInput(t *testing.T) {
	inputs := [][]byte{
		[]byte("2 R. N... 100644 100644 100644 h h R100 new\x00"),
		[]byte("x unsupported\x00"),
		[]byte("u XX N... 100644 100644 100644 100644 h h h path\x00"),
	}
	for _, input := range inputs {
		if _, err := ParseStatus(input); err == nil {
			t.Errorf("ParseStatus(%q) error = nil, want error", input)
		}
	}
}

func TestChangeIdentityIncludesScope(t *testing.T) {
	staged := Change{Path: "same.txt", Scope: ScopeStaged}
	unstaged := Change{Path: "same.txt", Scope: ScopeUnstaged}
	if staged.Identity() == unstaged.Identity() {
		t.Fatalf("identities unexpectedly equal: %#v", staged.Identity())
	}
}

func TestChangeIdentityTextAndSymbolsAreIndependentOfColor(t *testing.T) {
	for _, scope := range []Scope{ScopeStaged, ScopeUnstaged, ScopeUntracked} {
		if scope.String() == "" || scope.Symbol() == "" {
			t.Errorf("scope %d has empty representation", scope)
		}
	}
	for _, status := range []ChangeStatus{StatusAdded, StatusModified, StatusDeleted, StatusRenamed, StatusCopied, StatusTypeChanged, StatusUntracked, StatusUnmerged} {
		if status.String() == "" || status.Symbol() == "" {
			t.Errorf("status %d has empty representation", status)
		}
	}
	for _, conflict := range []ConflictKind{ConflictBothDeleted, ConflictAddedByUs, ConflictDeletedByThem, ConflictAddedByThem, ConflictDeletedByUs, ConflictBothAdded, ConflictBothModified} {
		if conflict.String() == "" {
			t.Errorf("conflict %q has empty representation", conflict)
		}
	}
}

func trackedRecord(xy, path string) string {
	return "1 " + xy + " N... 100644 100644 100644 head index " + path
}
