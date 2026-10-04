package kit

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/term"
)

// narrowWidth is where the two columns stop fitting; below it the pane is
// shown alone on demand.
const narrowWidth = 80

// PaneTab is the part a tab with the projects on the left and a program's
// pane on the right shares — Chat and Terminal: the split, the narrow and
// chosen-pane flags, the pane, its hold on the keys, and the host calls
// that are the same in both. A tab embeds it and calls Init once.
type PaneTab struct {
	Screen   Screen
	Rect     Rect
	Tick     int
	FullTerm bool // narrow terminal: only the pane is shown
	// PaneSel is panel 2 chosen by its number: lit, the keys still
	// lazychat's until Enter.
	PaneSel bool
	Pane    TermPane
	Capture *Capture
}

// Init wires the pane's capture of the keys; the PaneTab must already sit
// where it stays, inside its tab.
func (p *PaneTab) Init(screen Screen) {
	p.Screen = screen
	p.Capture = NewCapture(screen, &p.Pane, p.PaneRect)
}

func (p *PaneTab) Narrow() bool { return p.Rect.Cols < narrowWidth }

// LeftW is the list's width, the whole screen when narrow.
func (p *PaneTab) LeftW() int {
	if p.Narrow() {
		return p.Rect.Cols
	}
	return ListWidth(p.Rect.Cols)
}

// BodyH is the tab's height: the list's and the pane's.
func (p *PaneTab) BodyH() int { return max(8, p.Rect.Rows) }

// PaneRect is where the pane's inner area is on the screen: its size is
// what the ptys get.
func (p *PaneTab) PaneRect() Rect {
	if p.Narrow() {
		return Rect{X0: p.Rect.X0 + 1, Y0: p.Rect.Y0 + 1, Cols: p.Rect.Cols - 2, Rows: p.BodyH() - 2}
	}
	lw := p.LeftW()
	return Rect{X0: p.Rect.X0 + lw + 1, Y0: p.Rect.Y0 + 1, Cols: p.Rect.Cols - lw - 2, Rows: p.BodyH() - 2}
}

func (p *PaneTab) PaneSize() (cols, rows int) {
	r := p.PaneRect()
	return r.Cols, r.Rows
}

// SetRect takes the tab's new place; off a narrow screen the pane is never
// shown alone.
func (p *PaneTab) SetRect(r Rect) {
	p.Rect = r
	if !p.Narrow() {
		p.FullTerm = false
	}
}

// ToList gives the keys back to the list, the pane neither alone nor chosen.
func (p *PaneTab) ToList() { p.FullTerm, p.PaneSel = false, false }

// Choose lights panel 2 without going in: Enter goes in, so a number never
// lands the keys in the program.
func (p *PaneTab) Choose() { p.PaneSel, p.FullTerm = true, true }

// Point makes the pane show a session without giving it the keys.
func (p *PaneTab) Point(key string, s *term.Session) {
	cols, rows := p.PaneSize()
	p.Pane.Point(key, s, cols, rows)
}

// Note shows one line in the footer for a few seconds: the result of an
// action.
func (p *PaneTab) Note(format string, args ...any) { p.Screen.Note(format, args...) }

func (p *PaneTab) Ask(question string, yes func()) {
	p.Screen.Push(&Confirm{Question: question, Yes: yes})
}

// Ended lets the keys go when the shown program ended.
func (p *PaneTab) Ended(key string) {
	if p.Pane.Key == key {
		p.Capture.Drop()
	}
}

// Later runs f off the loop and then sends done, the tab's own message that
// its programs changed.
func (p *PaneTab) Later(f func(), done tea.Msg) {
	p.Screen.Queue(func() tea.Msg {
		f()
		return done
	})
}

// WheelPane is the wheel over the pane, d its rows: the program scrolls, or
// the emulator's history when it takes no mouse.
func (p *PaneTab) WheelPane(x, y, d int) {
	code := WheelUp
	if d > 0 {
		code = WheelDown
	}
	p.Pane.Mouse(p.PaneRect(), code, x, y, false)
}
