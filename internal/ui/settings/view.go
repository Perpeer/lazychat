package settings

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// hits are the zones the view marks: the settings' rows, and the values on
// the right numbered from 1 as headings are.
var hits = kit.Hits{Row: "setrow", Heading: "setval", Panels: []string{"setpanel-1", "setpanel-2"}}

func (s *Settings) leftW() int { return kit.Clamp(s.rect.Cols*32/100, 30, 44) }

func (s *Settings) View() string {
	h, lw := max(8, s.rect.Rows), s.leftW()
	all := s.settings()
	s.rows.ClampTo(len(all))
	var rows []string
	for i, st := range all {
		entry := []kit.TreeLine{
			{Styled: kit.StyleBold.Render(st.name), Plain: st.name},
			{Styled: kit.StyleDim.Render("  " + st.value()), Plain: "  " + st.value()},
		}
		b := kit.DrawEntry(entry, lw-2, i == s.rows.Sel, !s.onRight)
		rows = append(rows, kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Row, i), b, lw-2)...)
	}
	left := kit.Box(kit.PanelTitle(1, "settings"), rows, lw, h, !s.onRight, false)
	return kit.JoinHorizontal(hits.Panel(1, left), hits.Panel(2, s.values(s.rect.Cols-lw, h)))
}

// values is the right side: what the setting under the cursor does, and
// the values it can take, the chosen one marked ●.
func (s *Settings) values(w, h int) string {
	cur := s.current()
	inner := w - 2
	var lines []string
	if note := s.core.Settings.Note; note != "" {
		lines = append(lines, text.Wrap(note, inner-1, " ")...)
		lines = append(lines, "")
	}
	for _, l := range text.Wrap(cur.about, inner-1, " ") {
		lines = append(lines, kit.StyleDim.Render(l))
	}
	lines = append(lines, "")
	cs := cur.choices()
	s.choice.ClampTo(len(cs))
	for i, c := range cs {
		mark := "○ "
		switch {
		case cur.toggle && c.chosen:
			mark = "[✓] "
		case cur.toggle:
			mark = "[ ] "
		case c.chosen:
			mark = "● "
		}
		entry := []kit.TreeLine{{Styled: mark + c.name, Plain: mark + c.name}}
		if s.onRight && i == s.choice.Sel && c.about != "" {
			for _, l := range text.Wrap(c.about, inner-4, "  ") {
				entry = append(entry, kit.TreeLine{Styled: kit.StyleDim.Render(l), Plain: l})
			}
		}
		b := kit.DrawEntry(entry, inner, s.onRight && i == s.choice.Sel, s.onRight)
		lines = append(lines, kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Heading, i+1), b, inner)...)
	}
	return kit.Box(kit.PanelTitle(2, cur.name), lines, w, h, s.onRight, false)
}

// Mouse: a click on a setting selects it and shows its values; a click on a
// value chooses it; a click anywhere else in a box gives it the keys.
func (s *Settings) Mouse(msg tea.MouseMsg) tea.Cmd {
	if !kit.LeftClick(msg) {
		return nil
	}
	hit := hits.At(msg, len(s.settings()), len(s.current().choices()))
	switch hit.Kind {
	case kit.HitRow:
		s.rows.Sel, s.onRight = hit.N, false
	case kit.HitHeading:
		s.choice.Sel = hit.N - 1
		s.pick(hit.N - 1)
		s.onRight = s.current().toggle
	case kit.HitPanel: // the empty part of a box: it takes the keys, its selection stays
		if hit.N == 1 {
			s.onRight = false
		} else if !s.onRight {
			s.toValues()
		}
	}
	return nil
}
