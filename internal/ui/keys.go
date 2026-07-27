package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up              key.Binding
	Down            key.Binding
	Left            key.Binding
	Right           key.Binding
	Focus           key.Binding
	Open            key.Binding
	Back            key.Binding
	PreviousHunk    key.Binding
	NextHunk        key.Binding
	ToggleDensity   key.Binding
	ToggleLayout    key.Binding
	ErrorDetails    key.Binding
	FilterAll       key.Binding
	FilterStaged    key.Binding
	FilterChanges   key.Binding
	FilterUntracked key.Binding
	Refresh         key.Binding
	Help            key.Binding
	Quit            key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:              key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:            key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:            key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "scroll left")),
		Right:           key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "scroll right")),
		Focus:           key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "change focus")),
		Open:            key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view content")),
		Back:            key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "files")),
		PreviousHunk:    key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous hunk")),
		NextHunk:        key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next hunk")),
		ToggleDensity:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "changes/full")),
		ToggleLayout:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "inline/side-by-side")),
		ErrorDetails:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "error detail")),
		FilterAll:       key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "all")),
		FilterStaged:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "staged")),
		FilterChanges:   key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "changes")),
		FilterUntracked: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "untracked")),
		Refresh:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:            key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:            key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Focus, k.NextHunk, k.ToggleLayout, k.Refresh, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Focus, k.Open, k.Back},
		{k.PreviousHunk, k.NextHunk, k.ToggleDensity, k.ToggleLayout},
		{k.ErrorDetails},
		{k.FilterAll, k.FilterStaged, k.FilterChanges, k.FilterUntracked},
		{k.Refresh, k.Help, k.Quit},
	}
}
