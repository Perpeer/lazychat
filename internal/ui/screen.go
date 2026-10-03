package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// keyLogWidth bounds the key log, so a long note or a raw sequence cannot
// push the keys off the footer.
const keyLogWidth = 36

// The App is the screen every tab works through.
var _ kit.Screen = (*App)(nil)

func (a *App) Push(o kit.Overlay) { a.popups.Push(o) }

// Note shows one line in the footer for a few seconds: the result of an action.
func (a *App) Note(format string, args ...any) {
	a.note = fmt.Sprintf(format, args...)
	a.noteUntil = time.Now().Add(a.opts.NoteTime)
}

func (a *App) Queue(cmd tea.Cmd) {
	if a.pending == nil {
		a.pending = cmd
		return
	}
	a.pending = tea.Batch(a.pending, cmd)
}

// Send is safe from any goroutine; before the program starts it is dropped.
func (a *App) Send(msg tea.Msg) {
	if a.send != nil {
		a.send(msg)
	}
}

func (a *App) Capture(write func([]byte), leave func()) {
	if a.input == nil {
		return
	}
	a.input.Focus(write, leave, func(code, x, y int, release bool) {
		a.Send(kit.RawMouse{Code: code, X: x, Y: y, Release: release})
	})
}

func (a *App) Release() {
	if a.input != nil {
		a.input.Focus(nil, nil, nil)
	}
}

func (a *App) SetKitty(flags int) { a.kitty(flags) }

func (a *App) Switch(name string) {
	for i, t := range a.tabs {
		if t.Name() == name {
			a.switchTo(i)
			return
		}
	}
}

func (a *App) Size() (w, h int) { return a.width, a.height }

// Header is the help's first line; the tabs carry their titles in their frames.
func (a *App) Header(title string) string {
	clock := time.Now().Format("15:04:05") + "   Esc back "
	return kit.StyleHeader.Render(text.Pad(text.Fit(" "+title, a.width-text.Width(clock)-1), a.width-text.Width(clock)) + clock)
}

// FooterLine is the bottom row, from the project column on: the keys, or a
// recent note, then the tab's status and the key log at the right end.
func (a *App) FooterLine(keys []kit.Hint) string { return a.footerLine("", keys) }

// footerLine is FooterLine with the keys led by what they act on.
func (a *App) footerLine(lead string, keys []kit.Hint) string {
	indent := strings.Repeat(" ", railW)
	left := indent + lead + kit.RenderHints(keys)
	// The log gets what the keys and the status leave, and none at all
	// when that is too little to read.
	status := a.status()
	spare := a.width - text.Width(left) - text.Width(status) - 5
	if log := text.Fit(a.keyLog, min(keyLogWidth, spare)); spare >= 8 && log != "" {
		status = strings.TrimSpace(status + "   " + log)
	}
	// A one-row footer is the screen's last row: a fresh note and the
	// version end it, the note cut to what the keys leave.
	if a.footerRows < 2 {
		if tail := a.cornerTail(max(a.width-text.Width(left)-text.Width(status)-8, a.noteRoom())); tail != "" {
			status = strings.TrimSpace(status + "   " + tail)
		}
	}
	right := kit.StyleDim.Render(status + " ")
	room := a.width - text.Width(right) - 1
	if text.Width(left) > room {
		left = ansi.Truncate(left, room-1, "…")
	}
	return text.Pad(left, a.width-text.Width(right)) + "\x1b[0m" + right
}

// footerRight is the footer's right end: the tab's status and the key log,
// which gets what the keys leave on a row of left columns, and none at all
// when that is too little to read.
func (a *App) footerRight(left int) string {
	status := a.status()
	spare := a.width - left - text.Width(status) - 5
	if log := text.Fit(a.keyLog, min(keyLogWidth, spare)); spare >= 8 && log != "" {
		status = strings.TrimSpace(status + "   " + log)
	}
	return kit.StyleDim.Render(status + " ")
}

// footerNeeds is how many rows the footer takes: the keys' rows, and one
// more for a project's row.
func (a *App) footerNeeds(keys []kit.Hint) int {
	n := len(a.keyRows(keys))
	if len(a.projectKeys()) > 0 {
		n++
	}
	return n
}

// keyRows lays the keys out on as many rows as they need, so none is cut:
// one when they and a readable key log fit, else the first row keeps room
// for the log, as long as the widest one, and the rows under it take the
// whole width.
func (a *App) keyRows(keys []kit.Hint) [][]kit.Hint {
	lead := text.Width(a.leadText())
	if railW+lead+text.Width(kit.RenderHints(keys))+text.Width(a.status())+5+8 <= a.width || len(keys) == 0 {
		return [][]kit.Hint{keys}
	}
	room := a.width - railW - lead - keyLogWidth - text.Width(a.status()) - 6
	var out [][]kit.Hint
	for len(keys) > 0 {
		n := len(keys)
		for i := 1; i <= len(keys); i++ {
			if text.Width(kit.RenderHints(keys[:i])) > room {
				n = max(1, i-1)
				break
			}
		}
		out, keys = append(out, keys[:n]), keys[n:]
		room = a.width - railW - lead - 1
	}
	return out
}

// footer is the footer's rows, from the project column on: the keys, on
// as many rows as they need, or a recent note, with the status and the key
// log at the first row's right end; then the project's keys on the last
// row when the tab has them.
func (a *App) footer(keys []kit.Hint) string {
	indent := strings.Repeat(" ", railW)
	rows := a.footerRows
	var last string
	if below := a.projectKeys(); len(below) > 0 && rows > 1 {
		rows--
		last = indent + kit.StyleDim.Render("project: ") + kit.RenderHints(below)
		if text.Width(last) > a.width-1 {
			last = ansi.Truncate(last, a.width-2, "…")
		}
	}
	var top []string
	if lines := a.keyRows(keys); rows < 2 || len(lines) < 2 {
		top = []string{a.footerLine(a.leadText(), keys)}
	} else {
		lead := a.leadText()
		pad := strings.Repeat(" ", text.Width(lead))
		first := indent + lead + kit.RenderHints(lines[0])
		right := a.footerRight(text.Width(first))
		top = []string{text.Pad(first, a.width-text.Width(right)) + "\x1b[0m" + right}
		for _, l := range lines[1:min(len(lines), rows)] {
			top = append(top, indent+pad+kit.RenderHints(l))
		}
	}
	for len(top) < rows {
		top = append(top, "")
	}
	if a.footerRows > rows {
		top = append(top, last)
	}
	top[len(top)-1] = a.withVersion(top[len(top)-1])
	return strings.Join(top, "\n")
}

// withVersion puts a fresh note and the build's version at a footer row's
// right end, the screen's bottom-right corner, where the row leaves room:
// the note never takes the keys' place, so they stay put while it shows.
// v1.0(N) only, the hash being for the installer.
func (a *App) withVersion(row string) string {
	if a.footerRows < 2 {
		return row // a one-row footer has them after the key log
	}
	left := text.Width(strings.TrimRight(ansi.Strip(row), " "))
	tail := a.cornerTail(max(a.width-left-4, a.noteRoom()))
	if tail == "" {
		return row
	}
	room := a.width - text.Width(tail) - 1
	return text.Pad(ansi.Truncate(row, room, ""), room) + "\x1b[0m" + kit.StyleDim.Render(tail) + " "
}

// noteRoom is what a fresh note may take of the last row even when its
// keys fill it: the note whole, the row's tail cut for the few seconds it
// shows, and the rows above untouched. 0 with no note.
func (a *App) noteRoom() int {
	if !time.Now().Before(a.noteUntil) || a.note == "" {
		return 0
	}
	room := text.Width(a.note)
	if v := a.version(); v != "" {
		room += 3 + text.Width(v)
	}
	return min(room, a.width-railW-2)
}

// cornerTail is what ends the footer's last row in at most room columns:
// a fresh note, cut to fit, then the version; "" when neither fits.
func (a *App) cornerTail(room int) string {
	v := a.version()
	if text.Width(v) > room {
		v = ""
	}
	note := ""
	if time.Now().Before(a.noteUntil) && a.note != "" {
		gap := 0
		if v != "" {
			gap = 3
		}
		if free := room - text.Width(v) - gap; free >= 8 {
			note = text.Fit(a.note, free)
		}
	}
	switch {
	case note != "" && v != "":
		return note + "   " + v
	case note != "":
		return note
	}
	return v
}

// version is what the corner shows, v1.0(N); nothing for a build
// install.sh did not stamp, or when Settings turns it off.
func (a *App) version() string {
	v, _, _ := strings.Cut(a.opts.Version, " ")
	if v == "" || v == "dev" || (a.core.Settings != nil && a.core.Settings.NoVersion) {
		return ""
	}
	return "v" + v
}

// status is the footer's right end: what the mascot points at, when a
// session works or waits, then the tab's own status.
func (a *App) status() string {
	st := a.tab().Status()
	if m, ok := a.mascotState(); ok && m.Say != "" {
		say := m.Say
		if m.Mood == kit.Waiting {
			say = kit.StyleAccent.Render(say)
		}
		st = strings.TrimSpace(say + "   " + st)
	}
	return st
}

func (a *App) running() int {
	n := 0
	for _, t := range a.tabs {
		n += t.Running()
	}
	return n
}

// Quit always asks, from every tab, so a stray q never closes lazychat; the
// question names what runs, which ends with lazychat either way. A yes is a
// deliberate quit: the sessions are closed, not brought back next start, while
// an end without it (the window closed, a crash) still brings them back.
func (a *App) Quit() tea.Cmd {
	n := a.running()
	question := "quit lazychat?"
	if n > 0 {
		question = fmt.Sprintf("stop %d running session(s) and quit? They stay in the list; Enter or r resumes one.", n)
	}
	a.popups.Push(&kit.Confirm{Question: question, Yes: func() {
		if err := a.core.Store.ClearRunning(); err != nil {
			a.Note("quit: %v", err)
		}
		a.Queue(a.quitNow())
	}})
	return nil
}
