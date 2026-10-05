package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/status"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// railW is the rail's width: a box of four columns inside its frame.
const (
	railW     = 6
	railInner = railW - 2
)

// tabMsg is ⌘1 to ⌘9 from the router, numbered from 1.
type tabMsg int

// rail draws each tab as a small box with its name, top to bottom: the
// selected one framed and named in the accent colour, the rest dim. A tab
// that asks for it (Settings) sits at the rail's foot instead, apart from
// the tabs that hold work. Each box is a click target.
func (a *App) rail(h int) []string {
	var top, bottom []string
	for i, t := range a.tabs {
		if !a.shown(t) {
			continue
		}
		style := kit.StyleDim
		if i == a.active {
			style = kit.StyleAccent
		}
		name := text.Pad(text.Fit(t.Name(), railInner), railInner)
		// The selected box is drawn in double lines, its name in bold, so
		// the tab in use stands out from the thin dim ones.
		frame := [6]string{"┌", "─", "┐", "│", "└", "┘"}
		if i == a.active {
			style = style.Bold(true)
			frame = [6]string{"╔", "═", "╗", "║", "╚", "╝"}
		}
		box := kit.ZoneBlock(fmt.Sprintf("tab-%d", i), []string{
			style.Render(frame[0] + strings.Repeat(frame[1], railInner) + frame[2]),
			style.Render(frame[3] + name + frame[3]),
			style.Render(frame[4] + strings.Repeat(frame[1], railInner) + frame[5]),
		}, railW)
		if atBottom(t) {
			bottom = append(bottom, box...)
		} else {
			top = append(top, box...)
		}
	}
	// A newer release, or one installed and not run yet, is a box of its
	// own over Settings, in the running colour: the user found the
	// corner's ↑ too easy to miss. It is no tab: a click opens the popup.
	if a.updateShown() {
		style := kit.StyleBusy.Bold(true)
		name := text.Pad("new", railInner)
		box := kit.ZoneBlock(updateTabZone, []string{
			style.Render("┌" + strings.Repeat("─", railInner) + "┐"),
			style.Render("│" + name + "│"),
			style.Render("└" + strings.Repeat("─", railInner) + "┘"),
		}, railW)
		bottom = append(box, bottom...)
	}
	// The mascot stands at the rail's top, the tabs right under it,
	// Settings at its foot: its face, and under it the row of the keyboard
	// it types on, kept whether used or not, so nothing it does moves the
	// tabs. Its news plays inside its own frame: a question's call until it
	// is answered, a finished session's party until it works again — a new
	// prompt — or, while others still work, for a moment before it types on.
	// One badge on its top edge per session at work, up to three, right-
	// aligned as the question's mark is. With too little room it takes one
	// row, without badges.
	free := h - len(top) - len(bottom)
	var mascot []string
	if st, ok := a.mascotState(); ok && a.mascotOn() {
		blank := strings.Repeat(" ", railW)
		asking := len(st.Questions) > 0
		var face []string
		switch {
		case free >= mascotRows && asking:
			face = append(kit.MascotAsk(a.anim, railW), kit.AskKeyboard(railInner))
		case free >= mascotRows && (a.celebrating(st) || st.Cheer):
			face = append(kit.MascotParty(a.anim, railW), blank)
		case free >= mascotRows && st.Mood == kit.Working:
			face = kit.MascotTyping(a.anim, railW)
		case free >= mascotRows:
			face = append(kit.MascotFace(st.Mood, a.tick, railW), blank)
		case free >= 1 && st.Mood == kit.Working:
			face = []string{kit.MascotTypingLine(a.anim, railW)}
		case free >= 1:
			face = []string{kit.MascotLine(st.Mood, a.tick, railW)}
		}
		if len(face) >= 2 {
			keep := 0
			if asking {
				keep = 1 // the question's mark, by the corner
			}
			face = kit.MascotBadges(face, st.Busy, keep, railW)
		}
		mascot = kit.ZoneBlock(mascotZone, face, railW)
	}
	lines := append(mascot, top...)
	for len(lines)+len(bottom) < h {
		lines = append(lines, strings.Repeat(" ", railW))
	}
	lines = append(lines, bottom...)
	return lines[:min(h, len(lines))]
}

// mascotRows is the rows the mascot keeps above the tabs: three for its
// face, one for the keyboard it types on.
const mascotRows = 4

// mascotZone marks the mascot on the rail for clicks.
const mascotZone = "mascot"

// mascotTab is the tab whose sessions the mascot watches, and its index.
func (a *App) mascotTab() (kit.Mascot, int, bool) {
	for i, t := range a.tabs {
		if m, ok := t.(kit.Mascot); ok {
			return m, i, true
		}
	}
	return nil, 0, false
}

func (a *App) mascotState() (kit.MascotState, bool) {
	m, _, ok := a.mascotTab()
	if !ok {
		return kit.MascotState{}, false
	}
	return m.MascotState(), true
}

// mascotOn says the mascot is drawn; off, the footer still names what it
// would point at.
func (a *App) mascotOn() bool { return a.core.Settings == nil || !a.core.Settings.NoMascot }

// onMascot says a zero-based screen cell is on the mascot.
func (a *App) onMascot(x, y int) bool {
	return zone.Get(mascotZone).InBounds(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
}

// openMascot is a click on the mascot: Chat comes forward; with no
// question up it also opens the session the mascot points at, with the
// keys, and with one up it leaves that to the user, the asking sessions
// blinking in the list.
func (a *App) openMascot() {
	m, i, ok := a.mascotTab()
	if !ok {
		return
	}
	// Two or more sessions waiting: the inbox chooses; one: straight to it.
	if len(waiting(m.MascotState())) >= 2 {
		a.openInbox()
		return
	}
	a.switchTo(i)
	if st := m.MascotState(); len(st.Questions) == 0 {
		m.OpenMascot()
	}
}

// waiting are the sessions waiting on the user: asking first, then
// finished and not looked at, each group in the tree's order.
func waiting(st kit.MascotState) []kit.SessionNews {
	var out []kit.SessionNews
	for _, want := range []status.State{status.Asks, status.Done} {
		for _, s := range st.Sessions {
			if s.State == want {
				out = append(out, s)
			}
		}
	}
	return out
}

// openInbox lists the sessions waiting on the user in a finder; Enter
// opens the one chosen in Chat.
func (a *App) openInbox() {
	m, i, ok := a.mascotTab()
	if !ok {
		return
	}
	list := waiting(m.MascotState())
	if len(list) == 0 {
		a.Note("nothing waits on you")
		return
	}
	names := make([]string, len(list))
	for j, s := range list {
		names[j] = s.Name
	}
	f := kit.NewFinder("waiting on you", names, func(j int) {
		a.switchTo(i)
		m.OpenSession(list[j].Key)
	})
	f.Note = func(j int) string {
		word := "finished"
		if list[j].State == status.Asks {
			word = "asks"
		}
		return word + " · " + list[j].Project
	}
	a.Push(f)
}

// joinRail puts the rail to the left of the tab's rows.
func (a *App) joinRail(body string) string {
	rows := strings.Split(body, "\n")
	rail := a.rail(len(rows))
	for i := range rows {
		rows[i] = rail[i] + rows[i]
	}
	return strings.Join(rows, "\n")
}

// atBottom is a tab that sits at the rail's foot.
func atBottom(t kit.Tab) bool {
	b, ok := t.(interface{ AtBottom() bool })
	return ok && b.AtBottom()
}

// shown says t is on the rail. Chat, which the mascot and the other tabs
// send the user to, and Settings, where a tab is shown again, always are.
func (a *App) shown(t kit.Tab) bool {
	if t.Name() == "chat" || atBottom(t) || a.core.Settings == nil {
		return true
	}
	return a.core.Settings.Shown(t.Name())
}

// nextWorkTab is the tab Tab goes to (d = 1) or Shift+Tab (d = -1): the
// next or the previous of those that hold work, around the rail; Settings
// is a click or a ⌘ digit away.
func (a *App) nextWorkTab(d int) int {
	n := len(a.tabs)
	for step := 1; step <= n; step++ {
		if i := ((a.active+d*step)%n + n) % n; !atBottom(a.tabs[i]) && a.shown(a.tabs[i]) {
			return i
		}
	}
	return a.active
}

// railAt is the tab whose icon is under a zero-based screen cell, if any.
func (a *App) railAt(x, y int) (int, bool) {
	click := tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	for i := range a.tabs {
		if zone.Get(fmt.Sprintf("tab-%d", i)).InBounds(click) {
			return i, true
		}
	}
	return 0, false
}

// switchTo selects tab i; the tab leaving the screen gives up the keys.
func (a *App) switchTo(i int) {
	if i < 0 || i >= len(a.tabs) || i == a.active || !a.shown(a.tabs[i]) {
		return
	}
	a.tab().Blur()
	// The project under the cursor follows to the next tab, when both list
	// projects; what is under the project — a session, a shell — stays
	// each tab's own.
	var project string
	from, hasFrom := a.tab().(kit.ProjectTab)
	if hasFrom {
		project, hasFrom = from.CurrentProject()
	}
	a.active = i
	if to, ok := a.tab().(kit.ProjectTab); ok && hasFrom {
		a.pending = tea.Batch(a.pending, to.ShowProject(project))
	}
}

// animating says the mascot moves on its own beat: it types, celebrates a
// finished session, or calls out a question.
func (a *App) animating() bool {
	st, ok := a.mascotState()
	if !ok || !a.mascotOn() {
		return false
	}
	return st.Mood == kit.Working || a.celebrating(st) || len(st.Questions) > 0
}

// beating says the fast beat is wanted: by the mascot, or by the active tab
// while it shows motion of its own.
func (a *App) beating() bool {
	if t, ok := a.tab().(kit.Animator); ok && t.Animating() {
		return true
	}
	return a.animating()
}

// celebrating says a finished session waits to be looked at while nothing
// works: the mascot parties until it is looked at, or works again. With a
// session at work it types instead, the news a ✦ on its corner.
func (a *App) celebrating(st kit.MascotState) bool { return st.Finished > 0 && st.Mood != kit.Working }

// sound plays one of Lazy's sounds unless they are off.
func (a *App) sound(n sound.Name) {
	st := a.core.Settings
	if a.play == nil || st != nil && st.NoSounds {
		return
	}
	a.play(n)
}

// hearNews plays what changed since the last tick: a session that began to
// ask, one that finished — looked at or not — one that started on a prompt. The board decided both; this only listens.
func (a *App) hearNews() {
	st, ok := a.mascotState()
	if !ok {
		return
	}
	var names []sound.Name
	names, a.heard = newsSounds(a.heard, st.Sessions)
	for _, n := range names {
		a.sound(n)
	}
}

// newsSounds are the sounds for the sessions' changes since was, and the
// states to compare the next tick with. The first look hears nothing: what
// was already so when lazychat started is no news.
func newsSounds(was map[string]status.State, now []kit.SessionNews) ([]sound.Name, map[string]status.State) {
	next := make(map[string]status.State, len(now))
	var out []sound.Name
	for _, s := range now {
		next[s.Key] = s.State
		if was == nil || was[s.Key] == s.State {
			continue
		}
		switch s.State {
		case status.Working:
			// From asks it is the answer; only a new prompt starts the keys.
			if w, ok := was[s.Key]; ok && w != status.Asks {
				out = appendOnce(out, sound.Start)
			}
		case status.Asks:
			out = appendOnce(out, sound.Ask)
		case status.Done:
			out = appendOnce(out, sound.Done)
		case status.Idle:
			if was[s.Key] == status.Working {
				out = appendOnce(out, sound.Done)
			}
		}
	}
	return out, next
}

func appendOnce(names []sound.Name, n sound.Name) []sound.Name {
	for _, have := range names {
		if have == n {
			return names
		}
	}
	return append(names, n)
}
