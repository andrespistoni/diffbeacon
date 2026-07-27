package diff

import "testing"

func TestContentHunksContextAndNavigation(t *testing.T) {
	before := []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\n")
	after := []byte("A\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nL\n")
	document := NewTextDocument("many.txt", "", before, true, after, true)
	document.Patch = "@@ -1,2 +1,2 @@\n-a\n+A\n b\n@@ -11,2 +11,2 @@\n k\n-l\n+L\n"
	model := Build(document, Limits{ContextLines: 1})
	if len(model.Hunks) != 2 {
		t.Fatalf("hunks = %#v", model.Hunks)
	}
	if model.Hunks[0].ID != "hunk-1" || model.Hunks[1].ID != "hunk-2" {
		t.Fatalf("hunk IDs = %q, %q", model.Hunks[0].ID, model.Hunks[1].ID)
	}
	if got := model.NextHunk(-1); got != 0 {
		t.Fatalf("NextHunk(-1) = %d", got)
	}
	if got := model.PreviousHunk(0); got != 1 {
		t.Fatalf("PreviousHunk(0) = %d", got)
	}
	if len(model.ChangesRows()) >= len(model.FullRows) {
		t.Fatalf("changes rows = %d, full rows = %d", len(model.ChangesRows()), len(model.FullRows))
	}
}
