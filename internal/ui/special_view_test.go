package ui

import (
	"strings"
	"testing"

	"github.com/andrespistoni/diffbeacon/internal/app"
	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func TestBinarySpecialViewNeverRendersBytes(t *testing.T) {
	document := diffpkg.Document{
		Path: "image.bin", Kind: diffpkg.ContentBinary,
		Capability: diffpkg.Capability{HunksReason: "binary content is summarized; textual hunks are disabled"},
		Metadata:   diffpkg.Metadata{BeforeBytes: 3, AfterBytes: 4, Summary: "contains NUL bytes"},
	}
	state := specialState(document, gitpkg.Change{Path: document.Path, Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified})
	got := strings.Join(mustSpecial(t, state), "\n")
	assertGolden(t, "testdata/special/binary.golden", got)
	if strings.Contains(got, "\x00") || !strings.Contains(got, "never rendered") {
		t.Fatalf("binary view = %q", got)
	}
}

func TestConflictViewIsInformationalAndSanitizesContent(t *testing.T) {
	document := diffpkg.Document{
		Path: "conflict.txt", Kind: diffpkg.ContentConflict, AfterPresent: true,
		After:      "<<<<<<< ours\n\x1b[31mhostile\n=======\ntheirs\n>>>>>>> theirs\n",
		Capability: diffpkg.Capability{FullFile: true, HunksReason: "conflicted entries do not support partial hunks"},
		Metadata:   diffpkg.Metadata{AfterBytes: 50, Summary: "unmerged: both modified"},
	}
	change := gitpkg.Change{Path: document.Path, Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusUnmerged, Conflict: gitpkg.ConflictBothModified}
	got := strings.Join(mustSpecial(t, specialState(document, change)), "\n")
	assertGolden(t, "testdata/special/conflict.golden", got)
	if strings.Contains(got, "\x1b[31mhostile") || !strings.Contains(got, `\x1b[31mhostile`) || !strings.Contains(got, "side-by-side is not guaranteed") {
		t.Fatalf("conflict view = %q", got)
	}
}

func TestSubmoduleViewDoesNotEnterNestedRepository(t *testing.T) {
	document := diffpkg.Document{
		Path: "vendor/module", Kind: diffpkg.ContentSubmodule,
		Capability: diffpkg.Capability{HunksReason: "submodule content and partial hunks are not supported"},
		Metadata:   diffpkg.Metadata{Summary: "submodule: commit changed, tracked content modified"},
	}
	change := gitpkg.Change{Path: document.Path, Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusModified, Submodule: gitpkg.SubmoduleState{IsSubmodule: true, CommitChanged: true}}
	got := strings.Join(mustSpecial(t, specialState(document, change)), "\n")
	assertGolden(t, "testdata/special/submodule.golden", got)
	if !strings.Contains(got, "internals are not opened") || !strings.Contains(got, "Partial hunks: disabled") {
		t.Fatalf("submodule view = %q", got)
	}
}

func TestDegradedLimitExplainsBoundedFallback(t *testing.T) {
	document := diffpkg.Document{
		Path: "large.txt", Kind: diffpkg.ContentLimited,
		Capability: diffpkg.Capability{FullFileReason: "content exceeds 1048576-byte loading limit"},
		Metadata:   diffpkg.Metadata{AfterBytes: 2 << 20, Summary: "content exceeds loading limit"},
	}
	got := strings.Join(mustSpecial(t, specialState(document, gitpkg.Change{Path: document.Path, Scope: gitpkg.ScopeUntracked, Status: gitpkg.StatusUntracked})), "\n")
	if !strings.Contains(got, "deterministic loading limit") || !strings.Contains(got, "2097152 B") {
		t.Fatalf("degraded view = %q", got)
	}
}

func TestDeletedFileShowsPreviousContentAndAbsentNewSide(t *testing.T) {
	document := diffpkg.NewTextDocument("deleted.txt", "", []byte("kept from before\n"), true, nil, false)
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	change := gitpkg.Change{Path: document.Path, Scope: gitpkg.ScopeUnstaged, Status: gitpkg.StatusDeleted}
	state := app.NewModel()
	state.Snapshot.Changes = []gitpkg.Change{change}
	state.Selection, state.HasSelection = change.Identity(), true
	state.Detail = app.Detail{Identity: change.Identity(), Diff: &model}
	state.Density = app.DensityFullFile
	uiModel := New("repo", nil, nil)
	uiModel.state, uiModel.styles = state, newStyles(false)
	got := uiModel.renderContentSized(80, 10, app.LayoutSideBySide, "")
	if !strings.Contains(got, "Deleted file") || !strings.Contains(got, "kept from before") {
		t.Fatalf("deleted view = %q", got)
	}
}

func specialState(document diffpkg.Document, change gitpkg.Change) app.Model {
	model := diffpkg.Build(document, diffpkg.DefaultLimits())
	state := app.NewModel()
	state.Snapshot.Changes = []gitpkg.Change{change}
	state.Selection, state.HasSelection = change.Identity(), true
	state.Detail = app.Detail{Identity: change.Identity(), Diff: &model}
	state = app.Reduce(state, app.SetActiveHunk{})
	return state
}

func mustSpecial(t *testing.T, state app.Model) []string {
	t.Helper()
	lines, ok := renderSpecial(state)
	if !ok {
		t.Fatal("renderSpecial did not recognize special content")
	}
	return lines
}
