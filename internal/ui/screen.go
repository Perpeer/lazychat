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

// FooterLine is the bottom row, from the project column on: the keys, then
// the status area at the right end.
func (a *App) FooterLine(keys []kit.Hint) string { return a.footerLine("", keys) }

// footerLine is FooterLine with the keys led by what they act on: a
// one-row footer, the keys and the status area on the screen's last row.
func (a *App) footerLine(lead string, keys []kit.Hint) string {
	left := strings.Repeat(" ", railW) + lead + kit.RenderHints(keys)
	return a.withArea(left)
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

// areaNeed is what the status area wants of the last row to be read: the
// status, a short key log and the version.
func (a *App) areaNeed() int {
	return text.Width(a.status()) + 8 + text.Width(a.version()) + 9
}

// keyRows lays the keys out on as many rows as they need, so none is cut,
// each the whole width. With no project's row under them, the last row is
// also the status area's: when the keys leave it too little, the area gets
// a row of its own.
func (a *App) keyRows(keys []kit.Hint) [][]kit.Hint {
	lead := text.Width(a.leadText())
	room := a.width - railW - lead - 1
	out := kit.WrapHints(keys, room)
	if len(out) == 0 {
		return [][]kit.Hint{nil}
	}
	if len(a.projectKeys()) == 0 && text.Width(kit.RenderHints(out[len(out)-1]))+a.areaNeed() > room {
		out = append(out, nil)
	}
	return out
}

// footer is the footer's rows, from the project column on: the keys, on
// as many rows as they need, then the project's keys on the last row when
// the tab has them; the last row ends with the status area.
func (a *App) footer(keys []kit.Hint) string {
	indent := strings.Repeat(" ", railW)
	rows := a.footerRows
	if rows < 2 {
		return a.footerLine(a.leadText(), keys)
	}
	var last string
	if below := a.projectKeys(); len(below) > 0 {
		rows--
		last = indent + kit.StyleDim.Render("project: ") + kit.RenderHints(below)
	}
	lead := a.leadText()
	pad := strings.Repeat(" ", text.Width(lead))
	var top []string
	for i, l := range a.keyRows(keys) {
		if i >= rows {
			break
		}
		if i == 0 {
			top = append(top, indent+lead+kit.RenderHints(l))
		} else {
			top = append(top, indent+pad+kit.RenderHints(l))
		}
	}
	for len(top) < rows {
		top = append(top, "")
	}
	if a.footerRows > rows {
		top = append(top, last)
	}
	top[len(top)-1] = a.withArea(top[len(top)-1])
	return strings.Join(top, "\n")
}

// withArea ends a row with the status area at the screen's bottom-right
// corner; the row is cut to what the area leaves, a fresh note winning
// over the row's tail for the few seconds it shows.
func (a *App) withArea(row string) string {
	left := text.Width(strings.TrimRight(ansi.Strip(row), " "))
	tail := a.area(max(a.width-left-4, a.noteRoom()))
	if tail == "" {
		return text.Pad(ansi.Truncate(row, a.width-1, "…"), a.width)
	}
	room := a.width - text.Width(tail) - 1
	if text.Width(row) > room {
		row = ansi.Truncate(row, room-1, "…")
	}
	return text.Pad(row, room) + "\x1b[0m" + kit.StyleDim.Render(tail) + " "
}

// noteRoom is what a fresh note may take of the last row even when its
// keys fill it: the note and the version whole, the row's tail cut for the
// few seconds it shows. 0 with no note.
func (a *App) noteRoom() int {
	if !a.noteFresh() {
		return 0
	}
	room := text.Width(a.note)
	if v := a.version(); v != "" {
		room += 3 + text.Width(v)
	}
	return min(room, a.width-railW-2)
}

func (a *App) noteFresh() bool { return time.Now().Before(a.noteUntil) && a.note != "" }

// area is the status area in at most room columns, the one place in every
// tab for what goes on and what came of an action, read left to right: the
// mascot's line and the tab's status, a fresh note, the key log, then the
// version. What does not fit goes in the order the key log, the status,
// the note; the version stays while it fits.
func (a *App) area(room int) string {
	v := a.version()
	if text.Width(v) > room {
		v = ""
	}
	used := text.Width(v)
	take := func(s string, least int) string {
		if s == "" {
			return ""
		}
		gap := 0
		if used > 0 {
			gap = 3
		}
		free := room - used - gap
		if free < least {
			return ""
		}
		if text.Width(s) > free {
			s = ansi.Truncate(s, free-1, "…")
		}
		used += gap + text.Width(s)
		return s
	}
	note := ""
	if a.noteFresh() {
		note = take(a.note, 8)
	}
	status := take(a.status(), 4)
	log := take(text.Fit(a.keyLog, keyLogWidth), 8)
	var parts []string
	for _, p := range []string{status, note, log, v} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "   ")
}

// version is what the corner shows, v1.0.3, and in green the
// newer release when one is out; nothing for a build install.sh did not
// stamp, or when Settings turns it off.
func (a *App) version() string {
	v, _, _ := strings.Cut(a.opts.Version, " ")
	if v == "" || v == "dev" || (a.core.Settings != nil && a.core.Settings.NoVersion) {
		return ""
	}
	if a.latest != "" {
		return "v" + v + "  " + kit.StyleBusy.Render("↑ "+a.latest)
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
