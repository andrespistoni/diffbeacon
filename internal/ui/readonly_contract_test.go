package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReadOnlyContractHasNoMutationBindingsOrAffordances(t *testing.T) {
	model := New("repo", nil, nil)
	before := model.State()
	for _, input := range []rune{'s', 'u', 'S', 'U'} {
		updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{input}})
		if command != nil {
			t.Fatalf("key %q produced a command", input)
		}
		if got := updated.(Model).State(); !reflect.DeepEqual(got, before) {
			t.Fatalf("key %q changed application state", input)
		}
	}

	keys := DefaultKeyMap()
	var helpText strings.Builder
	for _, group := range keys.FullHelp() {
		for _, binding := range group {
			help := binding.Help()
			helpText.WriteString(help.Key)
			helpText.WriteByte(' ')
			helpText.WriteString(help.Desc)
			helpText.WriteByte('\n')
		}
	}
	lower := strings.ToLower(helpText.String() + model.View())
	for _, forbidden := range []string{"stage target", "unstage target", "stage all", "unstage all", "confirm", "discard", "edit file"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("read-only UI contains %q: %s", forbidden, lower)
		}
	}
}
