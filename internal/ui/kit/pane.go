package kit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/term"
	"lazychat/internal/ui/text"
)

// TermPane is the right side: the shown session's own screen, or, scrolled
// back, a window onto the rows that left it. Top is the history index of the
// first row shown; indices do not move as output arrives, so a reader who
// scrolled up keeps their place while claude streams below.
type TermPane struct {
	Session *term.Session
	Key     string // the record key of the shown session
	Live    bool   // following the bottom
	Top     int

	Sel CopyMode
}

// Rect is where the pane's inner area sits on the screen, zero-based: the
// column of its first cell, and its size, which is the ptys' size.
type Rect struct {
	X0, Y0     int
	Cols, Rows int
}

// Point makes the pane show a session and sizes its pty to the pane.
func (p *TermPane) Point(key string, s *term.Session, cols, rows int) {
	if p.Key != key {
		p.Live, p.Sel.Active = true, false
	}
	p.Session, p.Key = s, key
	if s != nil && s.Alive() {
		s.Resize(cols, rows)
	}
}

func (p *TermPane) Clear() { p.Session, p.Key, p.Live, p.Sel.Active = nil, "", true, false }

func (p *TermPane) Title() string {
	if p.Session == nil {
		return "session"
	}
	state := "running"
	if done, err := p.Session.Exit(); done {
		state = "ended"
		if err != nil {
			state = "ended · " + err.Error()
		}
	}
	t := p.Session.Project + " · " + p.Session.Name + " · " + state
	if p.Sel.Active {
		t += " · copy"
	} else if !p.Live {
		_, rows := p.Session.Size()
		t += fmt.Sprintf(" · ↑ %d", max(0, p.Session.Total()-rows-p.Top))
	}
	return t
}

// Scroll moves the window by n rows (negative is up, into the history) and
// goes back to following the bottom once it reaches it.
func (p *TermPane) Scroll(n int) {
	if p.Session == nil || p.Session.AltScreen() {
		return
	}
	_, rows := p.Session.Size()
	bottom := p.Session.Total() - rows
	if p.Live {
		p.Top = bottom
	}
	p.Top = Clamp(p.Top+n, 0, bottom)
	p.Live = p.Top >= bottom
}

// Scrollbar is where the view sits in the history, for h rows: the first row
// of the thumb and its length; length 0 when everything fits on screen.
func (p *TermPane) Scrollbar(h int) (from, length int) {
	if p.Session == nil || h <= 0 {
		return 0, 0
	}
	total := p.Session.Total()
	if total <= h {
		return 0, 0
	}
	top := p.Top
	if p.Live {
		top = total - h
	}
	length = max(1, h*h/total)
	from = top * (h - length) / max(1, total-h)
	return from, length
}

// ToLive is what typing into the session does: back to the bottom.
func (p *TermPane) ToLive() { p.Live, p.Sel.Active = true, false }

// View is the session's screen, row for row; the pane's inner size is the
// pty's size, so nothing needs wrapping. The program's cursor is drawn as a
// block, because the real cursor sits wherever Bubble Tea left it.
func (p *TermPane) View(w, h int, focused, blinkOn bool) []string {
	if p.Session == nil {
		return []string{"", StyleDim.Render(text.Fit(" no session shown — Enter on a project starts one, Enter on a session shows it", w))}
	}
	if p.Live && !p.Sel.Active {
		rows := strings.Split(p.Session.Render(), "\n")
		if len(rows) > h {
			rows = rows[:h]
		}
		if x, y, on := p.Session.Cursor(); on && p.Session.Alive() && y >= 0 && y < len(rows) && (!focused || blinkOn) {
			rows[y] = withCursor(rows[y], x, w, focused)
		}
		return ZoneBlock("term", rows, w)
	}
	if p.Live {
		p.Top = p.Session.Total() - h
	}
	styled, plain := p.Session.Rows(p.Top, p.Top+h)
	if p.Sel.Active {
		lo, hi := p.Sel.Span()
		for i := range styled {
			if abs := p.Top + i; abs >= lo && abs <= hi {
				styled[i] = StyleSel.Render(text.Pad(text.FitExact(plain[i], w), w))
			}
		}
	}
	return ZoneBlock("term", styled, w)
}

// Mouse routes a mouse event at screen cell (x, y), zero-based, to the shown
// session. A program on the alternate screen — claude — keeps its own history
// and scrolls it on the wheel, so the event goes to it, moved into the pane's
// coordinates; for anything else the pane scrolls the emulator's scrollback
// itself. Events outside the pane are dropped.
func (p *TermPane) Mouse(r Rect, code, x, y int, release bool) {
	s := p.Session
	if s == nil {
		return
	}
	px, py := x-r.X0, y-r.Y0
	if px < 0 || py < 0 || px >= r.Cols || py >= r.Rows {
		return
	}
	if s.AltScreen() {
		s.Mouse(code, px, py, release)
		return
	}
	switch code &^ (4 | 8 | 16) {
	case WheelUp:
		p.Scroll(-WheelRows)
	case WheelDown:
		p.Scroll(WheelRows)
	}
}

// ScrollBy scrolls the shown session by n rows from the keyboard: claude on
// the alternate screen gets wheel turns at the pane's centre, as a trackpad
// would send them; anything else scrolls the emulator's scrollback.
func (p *TermPane) ScrollBy(r Rect, n int) {
	s := p.Session
	if s == nil {
		return
	}
	if !s.AltScreen() {
		p.Scroll(n)
		return
	}
	code := WheelUp
	if n > 0 {
		code = WheelDown
	}
	for i := 0; i < max(1, abs(n)/WheelRows); i++ {
		s.Mouse(code, r.Cols/2, r.Rows/2, false)
	}
}

// WheelRows is how far one wheel notch scrolls the pane, as terminals do;
// PageRows is PgUp/PgDn (Fn+↑↓ on a MacBook): a few lines, so reading back
// through a stream does not skip what one is looking for.
const (
	WheelRows = 3
	PageRows  = 5
)

// The SGR mouse codes of one wheel notch.
const (
	WheelUp   = 64
	WheelDown = 65
)

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// withCursor paints the cell at column x as the cursor: a filled block when
// the session has the keys, an outline otherwise.
func withCursor(row string, x, w int, focused bool) string {
	if x < 0 || x >= w {
		return row
	}
	row = text.Pad(row, x+1)
	cell := ansi.Strip(ansi.Cut(row, x, x+1))
	if cell == "" {
		cell = " "
	}
	style := StyleCursor
	if !focused {
		style = StyleAccent.Underline(true)
	}
	return ansi.Truncate(row, x, "") + "\x1b[0m" + style.Render(cell) + ansi.TruncateLeft(row, x+1, "")
}
