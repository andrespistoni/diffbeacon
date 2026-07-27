package app

import (
	diffpkg "diffbeacon/internal/diff"
	gitpkg "diffbeacon/internal/git"
)

func reconcileSelection(oldChanges, newChanges []gitpkg.Change, selected gitpkg.ChangeIdentity, hasSelection bool) (gitpkg.ChangeIdentity, bool) {
	if len(newChanges) == 0 {
		return gitpkg.ChangeIdentity{}, false
	}
	if !hasSelection {
		return newChanges[0].Identity(), true
	}
	if containsIdentity(newChanges, selected) {
		return selected, true
	}
	for _, change := range newChanges {
		if change.Path == selected.Path {
			return change.Identity(), true
		}
	}

	oldIndex := identityIndex(oldChanges, selected)
	if oldIndex >= 0 {
		for index := oldIndex + 1; index < len(oldChanges); index++ {
			identity := oldChanges[index].Identity()
			if containsIdentity(newChanges, identity) {
				return identity, true
			}
		}
		for index := oldIndex - 1; index >= 0; index-- {
			identity := oldChanges[index].Identity()
			if containsIdentity(newChanges, identity) {
				return identity, true
			}
		}
		if oldIndex < len(newChanges) {
			return newChanges[oldIndex].Identity(), true
		}
		return newChanges[len(newChanges)-1].Identity(), true
	}
	return newChanges[0].Identity(), true
}

func containsIdentity(changes []gitpkg.Change, identity gitpkg.ChangeIdentity) bool {
	return identityIndex(changes, identity) >= 0
}

func identityIndex(changes []gitpkg.Change, identity gitpkg.ChangeIdentity) int {
	for index, change := range changes {
		if change.Identity() == identity {
			return index
		}
	}
	return -1
}

func normalizeHunk(active string, model *diffpkg.Model) string {
	if active == "" || model == nil {
		return ""
	}
	for _, hunk := range model.Hunks {
		if hunk.ID == active {
			return active
		}
	}
	return ""
}

func reconcileHunk(active string, previous, next *diffpkg.Model) string {
	if active == "" || previous == nil || next == nil {
		return ""
	}
	var old *diffpkg.Hunk
	for index := range previous.Hunks {
		if previous.Hunks[index].ID == active {
			old = &previous.Hunks[index]
			break
		}
	}
	if old == nil {
		return ""
	}
	for _, candidate := range next.Hunks {
		if candidate.BeforeStart == old.BeforeStart &&
			candidate.BeforeLineCount == old.BeforeLineCount &&
			candidate.AfterStart == old.AfterStart &&
			candidate.AfterLineCount == old.AfterLineCount {
			return candidate.ID
		}
	}
	return ""
}

func clampScroll(value, maximum int) int {
	if value < 0 || maximum <= 0 {
		return 0
	}
	if value >= maximum {
		return maximum - 1
	}
	return value
}

func rowCount(model *diffpkg.Model, density Density) int {
	if model == nil {
		return 0
	}
	if density == DensityChanges || (!model.Document.Capability.FullFile && model.Document.Capability.Hunks) {
		return len(model.ChangesRows())
	}
	return len(model.FullRows)
}
