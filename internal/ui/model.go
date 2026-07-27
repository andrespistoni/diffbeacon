package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"diffbeacon/internal/app"
	diffpkg "diffbeacon/internal/diff"
	watchpkg "diffbeacon/internal/watch"
)

const (
	defaultWidth  = 100
	defaultHeight = 30
)

type RefreshCoordinator interface {
	Begin(app.RefreshRequest) app.RefreshStarted
	Results() <-chan app.RefreshCompleted
}

type Model struct {
	state        app.Model
	coordinator  RefreshCoordinator
	watchSignals <-chan watchpkg.Signal
	repository   string
	keys         KeyMap
	help         help.Model
	styles       styles
	width        int
	height       int
	listScroll   int
	watchError   string
	errorDetails bool
}

func New(repository string, coordinator RefreshCoordinator, signals <-chan watchpkg.Signal) Model {
	helpModel := help.New()
	helpModel.ShowAll = false
	model := Model{
		state: app.NewModel(), coordinator: coordinator, watchSignals: signals,
		repository: repository, keys: DefaultKeyMap(), help: helpModel,
		styles: newStyles(true), width: defaultWidth, height: defaultHeight,
	}
	return model
}

func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{m.refreshCmd(app.RefreshInitial), m.waitRefreshCmd()}
	if m.watchSignals != nil {
		commands = append(commands, m.waitWatchCmd())
	}
	return tea.Batch(commands...)
}

func (m Model) State() app.Model { return m.state }

func (m Model) View() string {
	plan, helpView := m.viewLayout()
	width := plan.Width
	contentHeight := max(1, plan.BodyHeight-1)

	listTitle := " Files / Changes "
	contentTitle := " Diff / File "
	if m.state.Focus == app.FocusChanges {
		listTitle = "▶" + listTitle
	} else {
		contentTitle = "▶" + contentTitle
	}
	list, _ := renderChangeListViewport(m.state, m.styles, plan.ListWidth, contentHeight, m.listScroll)
	content := m.renderContentSized(plan.ContentWidth, contentHeight, plan.EffectiveLayout, plan.Reason)
	var body string
	if plan.Compact {
		title, visible := listTitle, list
		if m.state.Focus == app.FocusContent {
			title, visible = contentTitle, content
		}
		body = m.styles.panel.Width(plan.ContentWidth).Height(plan.BodyHeight).Render(m.styles.title.Render(title) + "\n" + fitWidthLines(visible, plan.ContentWidth, contentHeight))
	} else {
		left := m.styles.panel.Width(plan.ListWidth).Height(plan.BodyHeight).Render(m.styles.title.Render(listTitle) + "\n" + list)
		right := m.styles.panel.Width(plan.ContentWidth).Height(plan.BodyHeight).Render(m.styles.title.Render(contentTitle) + "\n" + fitWidthLines(content, plan.ContentWidth, contentHeight))
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}
	status := m.statusLine()
	return body + "\n" +
		m.styles.status.MaxWidth(width).Render(status) + "\n" + helpView
}

func (m Model) viewLayout() (layoutPlan, string) {
	width, height := m.width, m.height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}

	helpModel := m.help
	helpModel.Width = width
	helpView := helpModel.View(m.keys)
	helpHeight := max(1, strings.Count(helpView, "\n")+1)
	return planLayout(width, height, helpHeight, m.state.Layout), helpView
}

func (m Model) statusLine() string {
	parts := []string{
		filterName(m.state.Filter), layoutName(m.state.Layout), densityName(m.effectiveDensity()),
		"branch: " + nonEmpty(m.state.Snapshot.Revision.Branch, "…"),
		"repo: " + diffpkg.SafeDisplayText(filepath.Base(m.repository)),
	}
	if m.state.ActiveHunkID != "" {
		parts = append(parts, "hunk: "+m.state.ActiveHunkID)
	}
	if m.state.Progress.Refreshing {
		parts = append(parts, "⟳ refreshing ("+string(m.state.Progress.Reason)+")")
	}
	if m.state.Error != nil {
		parts = append(parts, "! "+m.state.Error.Summary)
	} else if m.watchError != "" {
		parts = append(parts, "! watcher: "+diffpkg.SafeDisplayText(m.watchError))
	}
	return strings.Join(parts, " · ")
}

func (m Model) renderContent() string {
	return m.renderContentSized(defaultWidth, defaultHeight, m.state.Layout, "")
}

func (m Model) renderContentSized(width, height int, layout app.Layout, layoutReason string) string {
	if problem := m.visibleError(); problem != nil && (m.errorDetails || m.state.Detail.Diff == nil) {
		return strings.Join(viewportLines(wrapLines(renderErrorView(problem, m.errorDetails), width), 0, 0, width, height), "\n")
	}
	if m.state.Detail.Error != nil {
		return strings.Join(renderErrorView(m.state.Detail.Error, false), "\n")
	}
	if m.state.Detail.Diff == nil {
		if m.state.Progress.Refreshing {
			return "⟳ Loading selected change…"
		}
		return "Select a change to inspect it."
	}
	prefix := make([]string, 0, 2)
	if layoutReason != "" {
		prefix = append(prefix, "! "+layoutReason)
	}
	if reason := m.state.Detail.Highlight.FallbackReason; reason != "" {
		prefix = append(prefix, "Plain-text highlighting fallback: "+diffpkg.SafeDisplayText(reason))
	}
	if notice := contentNotice(m.state); notice != "" {
		prefix = append(prefix, notice)
	}
	prefix = wrapLines(prefix, width)
	if lines, special := renderSpecial(m.state); special {
		lines = viewportLines(wrapLines(lines, width), m.state.ScrollY, m.state.ScrollX, width, max(1, height-len(prefix)))
		return strings.Join(append(prefix, lines...), "\n")
	}
	var lines []string
	vertical := renderScrollOffset(*m.state.Detail.Diff, m.effectiveDensity(), m.state.ScrollY)
	horizontal := m.state.ScrollX
	if layout == app.LayoutSideBySide {
		lines = renderSideBySide(m.state, m.styles, width)
		vertical = sideBySideScrollOffset(*m.state.Detail.Diff, m.effectiveDensity(), m.state.ScrollY)
		horizontal = 0
	} else {
		lines = renderInline(m.state, m.styles)
	}
	if len(lines) == 0 {
		return "No textual changes to display."
	}
	lines = viewportLines(lines, vertical, horizontal, width, max(1, height-len(prefix)))
	return strings.Join(append(prefix, lines...), "\n")
}

func (m Model) effectiveDensity() app.Density {
	if selectedUntracked(m.state) {
		return app.DensityFullFile
	}
	if m.state.Detail.Diff != nil && !m.state.Detail.Diff.Document.Capability.FullFile && m.state.Detail.Diff.Document.Capability.Hunks {
		return app.DensityChanges
	}
	return m.state.Density
}

type styles struct {
	color       bool
	panel       lipgloss.Style
	title       lipgloss.Style
	section     lipgloss.Style
	selected    lipgloss.Style
	status      lipgloss.Style
	addedLine   lipgloss.Style
	deletedLine lipgloss.Style
	addedMark   lipgloss.Style
	deletedMark lipgloss.Style
	hunk        lipgloss.Style
	muted       lipgloss.Style
}

func newStyles(color bool) styles {
	s := styles{color: color, panel: lipgloss.NewStyle().Border(lipgloss.NormalBorder())}
	if !color {
		s.title = lipgloss.NewStyle().Bold(true)
		s.section = lipgloss.NewStyle().Bold(true)
		s.selected = lipgloss.NewStyle().Bold(true)
		s.status = lipgloss.NewStyle()
		s.addedLine = lipgloss.NewStyle()
		s.deletedLine = lipgloss.NewStyle()
		s.addedMark = lipgloss.NewStyle()
		s.deletedMark = lipgloss.NewStyle()
		s.hunk = lipgloss.NewStyle().Bold(true)
		s.muted = lipgloss.NewStyle()
		return s
	}
	s.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#C4B5FD"})
	s.section = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"})
	s.selected = lipgloss.NewStyle().Bold(true).Background(lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1E3A5F"})
	s.status = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"})
	s.addedLine = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#E6FFEC", Dark: "#173B24"})
	s.deletedLine = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#FFEBE9", Dark: "#4A1F24"})
	s.addedMark = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#166534", Dark: "#86EFAC"})
	s.deletedMark = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#FCA5A5"})
	s.hunk = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"})
	s.muted = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	return s
}

func fitLines(value string, maximum int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > maximum {
		lines = lines[:maximum]
	}
	return strings.Join(lines, "\n")
}

func filterName(filter app.Filter) string {
	switch filter {
	case app.FilterStaged:
		return "Staged"
	case app.FilterChanges:
		return "Changes"
	case app.FilterUntracked:
		return "Untracked"
	default:
		return "All"
	}
}

func densityName(density app.Density) string {
	if density == app.DensityFullFile {
		return "Full file"
	}
	return "Changes"
}

func layoutName(layout app.Layout) string {
	if layout == app.LayoutSideBySide {
		return "Side-by-side"
	}
	return "Inline"
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (m Model) String() string { return fmt.Sprintf("DiffBeacon(%s)", m.repository) }
