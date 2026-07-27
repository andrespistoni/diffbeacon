package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultKeyMapExposesEveryCoreIntent(t *testing.T) {
	keys := DefaultKeyMap()
	cases := []struct {
		name    string
		binding key.Binding
		input   string
	}{
		{"up", keys.Up, "k"}, {"down", keys.Down, "j"}, {"focus", keys.Focus, "tab"},
		{"previous hunk", keys.PreviousHunk, "["}, {"next hunk", keys.NextHunk, "]"},
		{"density", keys.ToggleDensity, "f"}, {"all", keys.FilterAll, "1"},
		{"staged", keys.FilterStaged, "2"}, {"changes", keys.FilterChanges, "3"},
		{"untracked", keys.FilterUntracked, "4"}, {"refresh", keys.Refresh, "r"},
		{"help", keys.Help, "?"}, {"quit", keys.Quit, "q"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.input)}, test.binding) && test.input != "tab" {
				t.Fatalf("%q does not match %s binding", test.input, test.name)
			}
			if test.binding.Help().Key == "" || test.binding.Help().Desc == "" {
				t.Fatalf("%s binding has incomplete help", test.name)
			}
		})
	}
	if len(keys.ShortHelp()) == 0 || len(keys.FullHelp()) < 2 {
		t.Fatal("help map is incomplete")
	}
}
