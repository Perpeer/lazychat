package term

import (
	"fmt"
	"slices"
	"strings"
)

// resizeAnchored changes the emulator's height the way iTerm and Terminal do
// on the main screen: the text stays anchored at the bottom. Growing brings
// lines back from the scrollback above what is shown; shrinking drops blank
// rows at the bottom first and then scrolls the top rows into the
// scrollback. The emulator on its own adds blank rows under the text and
// crops from the bottom, which leaves an inline program like claude with an
// empty strip below it, or with its cursor inside older text. A program on
// the alternate screen redraws itself whole, so it is simply resized.
// The caller holds s.feed, so no output lands between the steps.
func (s *Session) resizeAnchored(oldRows, cols, rows int) {
	if s.emu.IsAltScreen() || rows == oldRows {
		s.emu.Resize(cols, rows)
		return
	}
	cur := s.emu.CursorPosition()
	if rows < oldRows {
		// The rows that have to stay: through the cursor and any text under it.
		last := cur.Y
		for y := oldRows - 1; y > cur.Y; y-- {
			if !s.blankRow(y) {
				last = y
				break
			}
		}
		if up := last + 1 - rows; up > 0 {
			// SU with the cursor anywhere saves the top rows to the scrollback.
			s.write(fmt.Sprintf("\x1b[%dS\x1b[%d;%dH", up, cur.Y-up+1, cur.X+1))
		}
		s.emu.Resize(cols, rows)
		return
	}
	sb := s.emu.Scrollback()
	lines := sb.Lines()
	back := min(rows-oldRows, len(lines))
	pulled := slices.Clone(lines[len(lines)-back:])
	kept := slices.Clone(lines[:len(lines)-back])
	sb.Clear()
	for _, l := range kept {
		sb.Push(l)
	}
	s.emu.Resize(cols, rows)
	if back == 0 {
		return
	}
	// SD moves what is shown down and leaves the cursor where it was; the
	// pulled lines fill the rows it opened, and the cursor follows its text.
	s.write(fmt.Sprintf("\x1b[%dT", back))
	for y, line := range pulled {
		for x := range min(len(line), cols) {
			c := line[x]
			s.emu.SetCell(x, y, &c)
		}
	}
	s.write(fmt.Sprintf("\x1b[%d;%dH", cur.Y+back+1, cur.X+1))
}

func (s *Session) write(seq string) { _, _ = s.emu.Write([]byte(seq)) }

func (s *Session) blankRow(y int) bool {
	for x := range s.emu.Width() {
		if c := s.emu.CellAt(x, y); c != nil && strings.TrimSpace(c.Content) != "" {
			return false
		}
	}
	return true
}
