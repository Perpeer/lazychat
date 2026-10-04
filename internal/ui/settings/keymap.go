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
	screen := func(s *Settings) kit.Screen { return s.screen }
	keyHelp := kit.HelpKey(screen, helpText)
	keyQuit := kit.QuitKey(kit.ListKeys.Quit, screen)
	keyBack := kit.BackKey(func(s *Settings) { s.onRight = false })
	panels := kit.PanelKeys(2, func(s *Settings, p int) tea.Cmd {
		if p == 1 {
			s.onRight = false
		} else if !s.onRight {
			s.toValues()
		}
		return nil
	}, kit.PanelNames.Settings)
	rowKeys = []binding{
		{Key: kit.SettingsKeys.Change, Run: act(func(s *Settings) { s.toValues() })},
		{Key: kit.SettingsKeys.RowUp, Run: act(func(s *Settings) { s.rows.Move(-1, len(s.settings())) })},
		{Key: kit.ListKeys.Down, Run: act(func(s *Settings) { s.rows.Move(1, len(s.settings())) })},
		keyHelp, keyQuit, keyBack,
	}
	rowKeys = append(rowKeys, panels...)
	valueKeys = []binding{
		{Key: kit.SettingsKeys.Choose, Run: act(func(s *Settings) { s.pick(s.choice.Sel); s.onRight = s.current().toggle })},
		{Key: kit.SettingsKeys.ValueUp, Run: act(func(s *Settings) { s.choice.Move(-1, len(s.current().choices())) })},
		{Key: kit.ListKeys.Down, Run: act(func(s *Settings) { s.choice.Move(1, len(s.current().choices())) })},
		{Key: kit.SettingsKeys.ValueBack, Run: act(func(s *Settings) { s.onRight = false })},
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
	return strings.Join(append(lines, kit.HelpFoot...), "\n")
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
