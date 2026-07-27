package app

func Reduce(model Model, message Message) Model {
	switch message := message.(type) {
	case RefreshStarted:
		if message.Generation >= model.Progress.Generation {
			model.Progress = Progress{Refreshing: true, Generation: message.Generation, Reason: message.Reason}
		}
	case RefreshCompleted:
		if message.Generation != model.Progress.Generation {
			return model
		}
		model.Progress.Refreshing = false
		if message.Err != nil {
			model.Error = makeAppError("refresh failed", message.Err)
			return model
		}
		oldVisible := model.VisibleChanges()
		newVisible := visibleChanges(message.Snapshot.Changes, model.Filter)
		if model.HasSelection && !containsIdentity(newVisible, model.Selection) {
			for _, change := range message.Snapshot.Changes {
				if change.Path == model.Selection.Path {
					if model.Filter != FilterAll {
						model.Filter = filterForScope(change.Scope)
						newVisible = visibleChanges(message.Snapshot.Changes, model.Filter)
					}
					break
				}
			}
		}
		model.Selection, model.HasSelection = reconcileSelection(oldVisible, newVisible, model.Selection, model.HasSelection)
		model.Snapshot = message.Snapshot
		model.Error = nil
		if model.HasSelection && message.Detail.Identity == model.Selection {
			previousDiff := model.Detail.Diff
			model.Detail = message.Detail
			model.ActiveHunkID = reconcileHunk(model.ActiveHunkID, previousDiff, model.Detail.Diff)
			model.ScrollY = clampScroll(model.ScrollY, rowCount(model.Detail.Diff, model.Density))
		} else {
			model.Detail = Detail{}
			model.ActiveHunkID = ""
			model.ScrollY = 0
		}
	case SelectChange:
		if containsIdentity(model.VisibleChanges(), message.Identity) {
			if !model.HasSelection || model.Selection != message.Identity {
				model.Detail = Detail{}
				model.ActiveHunkID = ""
				model.ScrollY, model.ScrollX = 0, 0
			}
			model.Selection, model.HasSelection = message.Identity, true
		}
	case SetFilter:
		oldVisible := model.VisibleChanges()
		model.Filter = message.Filter
		newVisible := model.VisibleChanges()
		model.Selection, model.HasSelection = reconcileSelection(oldVisible, newVisible, model.Selection, model.HasSelection)
		if !model.HasSelection || model.Detail.Identity != model.Selection {
			model.Detail = Detail{}
			model.ActiveHunkID = ""
			model.ScrollY, model.ScrollX = 0, 0
		}
	case SetActiveHunk:
		model.ActiveHunkID = normalizeHunk(message.HunkID, model.Detail.Diff)
	case SetScroll:
		model.ScrollY = clampScroll(message.Vertical, rowCount(model.Detail.Diff, model.Density))
		if message.Horizontal < 0 {
			model.ScrollX = 0
		} else {
			model.ScrollX = message.Horizontal
		}
	case SetLayout:
		model.Layout = message.Layout
	case SetDensity:
		model.Density = message.Density
		model.ScrollY = clampScroll(model.ScrollY, rowCount(model.Detail.Diff, model.Density))
	case SetFocus:
		model.Focus = message.Focus
	}
	return model
}
