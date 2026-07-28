package ui

import (
	"fmt"
	"strings"

	"github.com/andrespistoni/diffbeacon/internal/app"
	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
)

func renderChangeList(state app.Model, styles styles) string {
	lines, _ := changeListLines(state, styles)
	return strings.Join(lines, "\n")
}

func renderChangeListViewport(state app.Model, styles styles, width, height, vertical int) (string, int) {
	lines, selected := changeListLines(state, styles)
	vertical = min(max(0, vertical), max(0, len(lines)-height))
	if selected >= 0 && selected < vertical {
		vertical = selected
	} else if selected >= vertical+height {
		vertical = selected - height + 1
	}
	return strings.Join(viewportLines(lines, vertical, 0, width, height), "\n"), vertical
}

func changeListLines(state app.Model, styles styles) ([]string, int) {
	visible := state.VisibleChanges()
	if len(visible) == 0 {
		return []string{"No changes in " + strings.ToLower(filterName(state.Filter)) + ".", "Watching for repository updates."}, -1
	}

	var result []string
	selectedLine := -1
	groups := []struct {
		name  string
		match func(gitpkg.Scope) bool
	}{
		{"STAGED", func(scope gitpkg.Scope) bool { return scope == gitpkg.ScopeStaged }},
		{"CHANGES", func(scope gitpkg.Scope) bool { return scope == gitpkg.ScopeUnstaged || scope == gitpkg.ScopeUntracked }},
	}
	for _, group := range groups {
		var scoped []gitpkg.Change
		for _, change := range visible {
			if group.match(change.Scope) {
				scoped = append(scoped, change)
			}
		}
		if len(scoped) == 0 {
			continue
		}
		if len(result) > 0 {
			result = append(result, "")
		}
		result = append(result, styles.section.Render(fmt.Sprintf("%s (%d)", group.name, len(scoped))))
		for _, change := range scoped {
			line := formatChange(change, state.HasSelection && change.Identity() == state.Selection)
			if state.HasSelection && change.Identity() == state.Selection {
				selectedLine = len(result)
				line = styles.selected.Render(line)
			}
			result = append(result, line)
		}
	}
	return result, selectedLine
}

func formatChange(change gitpkg.Change, selected bool) string {
	marker := " "
	if selected {
		marker = ">"
	}
	path := diffpkg.SafeDisplayText(change.Path)
	if change.OldPath != "" {
		path = diffpkg.SafeDisplayText(change.OldPath) + " -> " + path
	}
	details := make([]string, 0, 2)
	if change.Conflict != "" {
		details = append(details, "conflict: "+change.Conflict.String())
	}
	if change.Submodule.IsSubmodule {
		state := "submodule"
		var flags []string
		if change.Submodule.CommitChanged {
			flags = append(flags, "commit")
		}
		if change.Submodule.TrackedModified {
			flags = append(flags, "modified")
		}
		if change.Submodule.UntrackedPresent {
			flags = append(flags, "untracked")
		}
		if len(flags) > 0 {
			state += ": " + strings.Join(flags, ",")
		}
		details = append(details, state)
	}
	if len(details) > 0 {
		path += " (" + strings.Join(details, "; ") + ")"
	}
	return fmt.Sprintf("%s %s %s %s", marker, change.Scope.Symbol(), change.Status.Symbol(), path)
}

func selectedUntracked(state app.Model) bool {
	return state.HasSelection && state.Selection.Scope == gitpkg.ScopeUntracked
}
