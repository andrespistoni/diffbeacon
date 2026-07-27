package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"diffbeacon/internal/app"
	gitpkg "diffbeacon/internal/git"
	watchpkg "diffbeacon/internal/watch"
)

type refreshStreamClosedMsg struct{}
type watchStreamClosedMsg struct{}
type watchSignalMsg struct{ signal watchpkg.Signal }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.help.Width = message.Width
		return m, nil
	case app.RefreshStarted:
		m.state = app.Reduce(m.state, message)
		return m, nil
	case app.RefreshCompleted:
		accepted := message.Generation == m.state.Progress.Generation
		m.state = app.Reduce(m.state, message)
		if accepted && message.Err == nil && m.state.HasSelection && m.state.Detail.Identity != m.state.Selection {
			// The selected entry disappeared while this refresh was loading it.
			// The reducer deterministically selected a neighbor, so immediately
			// load that neighbor instead of leaving a blank detail until another
			// filesystem event or periodic reconciliation happens.
			return m, tea.Batch(m.waitRefreshCmd(), m.refreshCmd(app.RefreshWatch))
		}
		return m, m.waitRefreshCmd()
	case refreshStreamClosedMsg:
		return m, nil
	case watchSignalMsg:
		if message.signal.Err != nil {
			m.watchError = message.signal.Err.Error()
		} else {
			m.watchError = ""
		}
		reason := app.RefreshWatch
		if message.signal.Reason == watchpkg.ReasonReconcile {
			reason = app.RefreshReconcile
		}
		return m, tea.Batch(m.refreshCmd(reason), m.waitWatchCmd())
	case watchStreamClosedMsg:
		m.watchSignals = nil
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(message)
	}
	return m, nil
}

func (m Model) updateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.errorDetails && (key.Matches(message, m.keys.ErrorDetails) || key.Matches(message, m.keys.Back)) {
		m.errorDetails = false
		return m, nil
	}
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(message, m.keys.Refresh):
		return m, m.refreshCmd(app.RefreshManual)
	case key.Matches(message, m.keys.ErrorDetails):
		if m.visibleError() != nil {
			m.errorDetails = !m.errorDetails
		}
		return m, nil
	case key.Matches(message, m.keys.Focus):
		if m.state.Focus == app.FocusChanges {
			m.state = app.Reduce(m.state, app.SetFocus{Focus: app.FocusContent})
		} else {
			m.state = app.Reduce(m.state, app.SetFocus{Focus: app.FocusChanges})
		}
		return m, nil
	case key.Matches(message, m.keys.Open):
		m.state = app.Reduce(m.state, app.SetFocus{Focus: app.FocusContent})
		return m, nil
	case key.Matches(message, m.keys.Back):
		if m.state.ActiveHunkID != "" {
			m.state = app.Reduce(m.state, app.SetActiveHunk{})
			return m, nil
		}
		m.state = app.Reduce(m.state, app.SetFocus{Focus: app.FocusChanges})
		return m, nil
	case key.Matches(message, m.keys.FilterAll):
		return m.setFilter(app.FilterAll)
	case key.Matches(message, m.keys.FilterStaged):
		return m.setFilter(app.FilterStaged)
	case key.Matches(message, m.keys.FilterChanges):
		return m.setFilter(app.FilterChanges)
	case key.Matches(message, m.keys.FilterUntracked):
		return m.setFilter(app.FilterUntracked)
	case key.Matches(message, m.keys.ToggleDensity):
		if selectedUntracked(m.state) || (m.state.Detail.Diff != nil && !m.state.Detail.Diff.Document.Capability.FullFile) {
			return m, nil
		}
		density := app.DensityFullFile
		if m.state.Density == app.DensityFullFile {
			density = app.DensityChanges
		}
		m.state = app.Reduce(m.state, app.SetDensity{Density: density})
		return m, nil
	case key.Matches(message, m.keys.ToggleLayout):
		layout := app.LayoutSideBySide
		if m.state.Layout == app.LayoutSideBySide {
			layout = app.LayoutInline
		}
		m.state = app.Reduce(m.state, app.SetLayout{Layout: layout})
		return m, nil
	case key.Matches(message, m.keys.PreviousHunk):
		return m.moveHunk(false)
	case key.Matches(message, m.keys.NextHunk):
		return m.moveHunk(true)
	case key.Matches(message, m.keys.Up):
		if m.state.Focus == app.FocusContent {
			m.state = app.Reduce(m.state, app.SetScroll{Vertical: m.state.ScrollY - 1, Horizontal: m.state.ScrollX})
			return m, nil
		}
		return m.moveSelection(-1)
	case key.Matches(message, m.keys.Down):
		if m.state.Focus == app.FocusContent {
			m.state = app.Reduce(m.state, app.SetScroll{Vertical: m.state.ScrollY + 1, Horizontal: m.state.ScrollX})
			return m, nil
		}
		return m.moveSelection(1)
	case key.Matches(message, m.keys.Left):
		if m.state.Focus == app.FocusContent {
			m.state = app.Reduce(m.state, app.SetScroll{Vertical: m.state.ScrollY, Horizontal: m.state.ScrollX - 4})
		}
		return m, nil
	case key.Matches(message, m.keys.Right):
		if m.state.Focus == app.FocusContent {
			m.state = app.Reduce(m.state, app.SetScroll{Vertical: m.state.ScrollY, Horizontal: m.state.ScrollX + 4})
		}
		return m, nil
	}
	return m, nil
}

func (m Model) setFilter(filter app.Filter) (tea.Model, tea.Cmd) {
	previous := m.state.Selection
	m.state = app.Reduce(m.state, app.SetFilter{Filter: filter})
	if m.state.HasSelection && (m.state.Selection != previous || m.state.Detail.Identity != m.state.Selection) {
		return m, m.refreshCmd(app.RefreshManual)
	}
	return m, nil
}

func (m Model) moveSelection(delta int) (tea.Model, tea.Cmd) {
	changes := m.state.VisibleChanges()
	if len(changes) == 0 {
		return m, nil
	}
	index := changeIndex(changes, m.state.Selection)
	if index < 0 {
		index = 0
	} else {
		index = min(max(0, index+delta), len(changes)-1)
	}
	identity := changes[index].Identity()
	if m.state.HasSelection && identity == m.state.Selection {
		return m, nil
	}
	m.state = app.Reduce(m.state, app.SelectChange{Identity: identity})
	plan, _ := m.viewLayout()
	_, m.listScroll = renderChangeListViewport(m.state, m.styles, plan.ListWidth, max(1, plan.BodyHeight-1), m.listScroll)
	return m, m.refreshCmd(app.RefreshManual)
}

func (m Model) moveHunk(forward bool) (tea.Model, tea.Cmd) {
	model := m.state.Detail.Diff
	if model == nil || len(model.Hunks) == 0 {
		return m, nil
	}
	current := -1
	for index := range model.Hunks {
		if model.Hunks[index].ID == m.state.ActiveHunkID {
			current = index
			break
		}
	}
	next := model.PreviousHunk(current)
	if forward {
		next = model.NextHunk(current)
	}
	if next < 0 {
		return m, nil
	}
	m.state = app.Reduce(m.state, app.SetActiveHunk{HunkID: model.Hunks[next].ID})
	m.state = app.Reduce(m.state, app.SetScroll{Vertical: hunkOffset(*model, next, m.effectiveDensity()), Horizontal: m.state.ScrollX})
	return m, nil
}

func (m Model) refreshCmd(reason app.RefreshReason) tea.Cmd {
	if m.coordinator == nil {
		return nil
	}
	request := app.RefreshRequest{Reason: reason, Selection: m.state.Selection, HasSelection: m.state.HasSelection}
	return func() tea.Msg { return m.coordinator.Begin(request) }
}

func (m Model) waitRefreshCmd() tea.Cmd {
	if m.coordinator == nil {
		return nil
	}
	results := m.coordinator.Results()
	return func() tea.Msg {
		result, ok := <-results
		if !ok {
			return refreshStreamClosedMsg{}
		}
		return result
	}
}

func (m Model) waitWatchCmd() tea.Cmd {
	signals := m.watchSignals
	if signals == nil {
		return nil
	}
	return func() tea.Msg {
		signal, ok := <-signals
		if !ok {
			return watchStreamClosedMsg{}
		}
		return watchSignalMsg{signal: signal}
	}
}

func changeIndex(changes []gitpkg.Change, identity gitpkg.ChangeIdentity) int {
	for index, change := range changes {
		if change.Identity() == identity {
			return index
		}
	}
	return -1
}
