package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

type binding = kit.Binding[*Settings]

var act = kit.Act[*Settings]

// rowKeys are the keys over the settings, valueKeys over one's values.
var rowKeys, valueKeys []binding

func init() {
	keyHelp := binding{Keys: []string{"?"}, Hint: kit.Hint{Key: "?", Does: "help"}, Run: act(func(s *Settings) {
		s.screen.Push(kit.NewPager("keys", helpText(), s.screen.Header, s.screen.FooterLine))
	})}
	keyQuit := binding{Keys: []string{"q"}, Hint: kit.Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked", Run: func(s *Settings) tea.Cmd { return s.screen.Quit() }}
	keyBack := kit.BackKey(func(s *Settings) { s.onRight = false })
	panels := kit.PanelKeys(2, func(s *Settings, p int) tea.Cmd {
		if p == 1 {
			s.onRight = false
		} else if !s.onRight {
			s.toValues()
		}
		return nil
	}, "1 the settings, 2 the values of the one under the cursor")
	rowKeys = []binding{
		{Keys: []string{"enter", "right", "l"}, Hint: kit.Hint{Key: "enter", Does: "change"}, Help: "go to the setting's values on the right; →  and l too", Run: act(func(s *Settings) { s.toValues() })},
		{Keys: []string{"up", "k"}, Name: "↑↓ j k", Help: "move over the settings", Run: act(func(s *Settings) { s.rows.Move(-1, len(s.settings())) })},
		{Keys: []string{"down", "j"}, Run: act(func(s *Settings) { s.rows.Move(1, len(s.settings())) })},
		keyHelp, keyQuit, keyBack,
	}
	rowKeys = append(rowKeys, panels...)
	valueKeys = []binding{
		{Keys: []string{"enter", " "}, Hint: kit.Hint{Key: "enter", Does: "choose"}, Help: "make the value under the cursor the setting's, saved at once; in tabs, show or hide the one under the cursor and stay", Run: act(func(s *Settings) { s.pick(s.choice.Sel); s.onRight = s.current().toggle })},
		{Keys: []string{"up", "k"}, Name: "↑↓ j k", Help: "move over the values", Run: act(func(s *Settings) { s.choice.Move(-1, len(s.current().choices())) })},
		{Keys: []string{"down", "j"}, Run: act(func(s *Settings) { s.choice.Move(1, len(s.current().choices())) })},
		{Keys: []string{"esc", "left", "h"}, Hint: kit.Hint{Key: "esc", Does: "back"}, Help: "back to the settings, nothing changed; ← and h too", Run: act(func(s *Settings) { s.onRight = false })},
		keyHelp, keyQuit, keyBack,
	}
	valueKeys = append(valueKeys, panels...)
}

func (s *Settings) bindings() []binding {
	if s.onRight {
		return valueKeys
	}
	return rowKeys
}

func (s *Settings) Key(msg tea.KeyMsg) tea.Cmd { return kit.Dispatch(s.bindings(), msg.String(), s) }

func (s *Settings) Footer() []kit.Hint { return kit.FooterHints(s.bindings()) }

func helpText() string {
	lines := []string{
		"The Settings tab: what you set for this machine, kept in ~/.lazychat/settings.json.",
		"",
	}
	lines = append(lines, kit.HelpSection("Settings", rowKeys)...)
	lines = append(lines, kit.HelpSection("Values", valueKeys)...)
	return strings.Join(append(lines, kit.WorkspaceHelp...), "\n")
}

// toValues moves the keys to the current setting's values, the cursor on
// the chosen one; a toggle row's start at the top, as any may be on.
func (s *Settings) toValues() {
	s.onRight = true
	s.choice.Sel = 0
	for i, c := range s.current().choices() {
		if c.chosen && !s.current().toggle {
			s.choice.Sel = i
		}
	}
}
