package ui

import (
	"fmt"
	"strings"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
)

func renderSpecial(state app.Model) ([]string, bool) {
	if state.Detail.Diff == nil {
		return nil, false
	}
	document := state.Detail.Diff.Document
	if !state.Detail.Diff.Degraded && document.Kind == diffpkg.ContentText {
		return nil, false
	}
	kind := string(document.Kind)
	if kind == "" {
		kind = "degraded"
	}
	lines := []string{
		"! Special/degraded view: " + kind,
		"Path: " + diffpkg.SafeDisplayText(document.Path),
	}
	if document.Metadata.Summary != "" {
		lines = append(lines, "Summary: "+diffpkg.SafeDisplayText(document.Metadata.Summary))
	}
	lines = append(lines, fmt.Sprintf("Size: before %d B · after %d B", document.Metadata.BeforeBytes, document.Metadata.AfterBytes))
	if document.Metadata.BeforeMode != "" || document.Metadata.AfterMode != "" {
		lines = append(lines, "Mode: "+nonEmpty(document.Metadata.BeforeMode, "absent")+" -> "+nonEmpty(document.Metadata.AfterMode, "absent"))
	}
	reason := nonEmpty(document.Capability.HunksReason, state.Detail.Diff.Reason)
	lines = append(lines, "Partial hunks: disabled — "+nonEmpty(diffpkg.SafeDisplayText(reason), "no textual hunk capability"))

	switch document.Kind {
	case diffpkg.ContentBinary:
		lines = append(lines, "Binary bytes are never rendered as terminal text.")
	case diffpkg.ContentSubmodule:
		lines = append(lines, "Submodule internals are not opened or modified.")
	case diffpkg.ContentConflict:
		lines = append(lines, "Conflict resolution is outside DiffBeacon; side-by-side is not guaranteed.")
		if document.AfterPresent && document.After != "" {
			lines = append(lines, "", "Working tree content (informational):")
			for _, line := range strings.Split(document.After, "\n") {
				lines = append(lines, diffpkg.SafeDisplayText(line))
			}
		}
	case diffpkg.ContentLimited:
		lines = append(lines, "Content was not retained after the deterministic loading limit was reached.")
	case diffpkg.ContentTypeChange:
		lines = append(lines, "The filesystem type changed; no textual comparison is inferred.")
	}
	return lines, true
}

func contentNotice(state app.Model) string {
	if state.Detail.Diff != nil {
		document := state.Detail.Diff.Document
		if !document.Capability.FullFile && document.Capability.Hunks {
			return "Full-file view unavailable: " + nonEmpty(diffpkg.SafeDisplayText(document.Capability.FullFileReason), "content exceeds the full-file budget") + ". Showing changes only."
		}
	}
	change, ok := selectedChange(state)
	if !ok {
		return ""
	}
	if change.Status == gitpkg.StatusDeleted {
		return "Deleted file: showing previous content; the new side is absent."
	}
	if change.Scope == gitpkg.ScopeUntracked {
		return "Untracked file: complete content is shown; the previous side is absent."
	}
	return ""
}

func selectedChange(state app.Model) (gitpkg.Change, bool) {
	for _, change := range state.Snapshot.Changes {
		if state.HasSelection && change.Identity() == state.Selection {
			return change, true
		}
	}
	return gitpkg.Change{}, false
}
