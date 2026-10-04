package chat

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/sound"
	"lazychat/internal/ui/kit"
)

// Routing: what arrives — keys, the mouse, messages — becomes operations on
// the model, the pane and the actions.

// hits are the zones the view marks.
var hits = kit.Hits{Row: "row", Heading: "proj", Pane: "term", Panels: []string{"cpanel-1", "cpanel-2", "cpanel-3"}}

func (c *Chat) Update(msg tea.Msg) tea.Cmd {
	c.mascot.valid = false
	if c.Capture.Update(msg) {
		return nil
	}
	switch msg := msg.(type) {
	case kit.Beat:
		c.beat = msg.N
		return nil
	case kit.Tick:
		c.Tick = msg.N
		c.list.tick = msg.N
		// On the first beat, when the pane has its size: the sessions that
		// ran when lazychat last quit come back.
		if !c.resumed {
			c.resumed = true
			if n := c.act.ResumeRunning(); n > 0 {
				// Shown, not entered: the list keeps the keys at start.
				c.Capture.Drop()
				c.Screen.Note("resumed %d session(s) that ran when lazychat quit", n)
			}
		}
		c.act.LearnIDs()
		var failed tea.Cmd
		if c.watchSessions() {
			failed = playSound(sound.Error)
		}
		var clock tea.Cmd
		if msg.N%2 == 0 {
			clock = c.readClocks()
		}
		return tea.Batch(c.reportTick(msg.N), clock, failed)
	case termMsg:
		c.act.Live.AckAll()
		c.act.Reap()
	case kit.ProjectAction:
		c.projectAction(msg)
	case draftSentMsg:
		c.draftSent(msg)
	case reportMsg:
		return c.reported(msg)
	case clocksMsg:
		c.clocked(msg)
	case kit.CmdEnter:
		if c.drafting {
			return c.sendDraft()
		}
	}
	return nil
}

// projectAction is a project row's shift key, from this tab or another:
// Chat owns the projects' sessions, so it opens, edits and removes them.
// Opening brings Chat forward, where the new project's sessions start; the
// others ask over whichever tab is shown.
func (c *Chat) projectAction(msg kit.ProjectAction) {
	if msg.Do == "open" {
		c.Screen.Switch(c.Name())
		c.act.AddProject()
		return
	}
	p, ok := c.core.Store.ProjectNamed(msg.Project)
	if !ok {
		return
	}
	switch msg.Do {
	case "edit":
		c.act.EditProject(p)
	case "remove":
		c.act.RemoveProject(p)
	}
}

func (c *Chat) Key(msg tea.KeyMsg) tea.Cmd {
	c.mascot.valid = false
	if c.drafting {
		return c.draftKey(msg)
	}
	if !(c.rep.shown && c.repFocus) {
		c.list.scroll.Follow()
	}
	// A move in the tree while the report shows reads the new session.
	return tea.Batch(kit.Dispatch(c.bindings(), msg.String(), c), c.reportFollow())
}

// selectShown moves the cursor to the shown session, so leaving the terminal
// lands on what was just in use.
func (c *Chat) selectShown() {
	if c.Pane.Session == nil {
		return
	}
	c.tree.SelectSession(c.Pane.Key)
}

func (c *Chat) move(d int) {
	if c.Capture.Held() {
		return
	}
	c.tree.Step(d)
	c.followCursor()
}

// moveSession and moveProject reorder the tree; the cursor stays on its
// session, wherever that ends up.
func (c *Chat) moveSession(d int) {
	if err := c.tree.MoveSession(d); err != nil {
		c.Screen.Note("move: %v", err)
	}
}

func (c *Chat) moveProject(d int) {
	if err := c.tree.MoveProject(d); err != nil {
		c.Screen.Note("move: %v", err)
	}
}

// carry is a step of move mode: a session among its project's, or with
// M the cursor's project among the projects.
func (c *Chat) carry(d int) {
	if c.tree.Whole {
		c.moveProject(d)
	} else {
		c.moveSession(d)
	}
}

// followCursor makes the pane show the record under the cursor when it runs,
// so browsing the list switches terminals.
func (c *Chat) followCursor() {
	if r, ok := c.tree.Session(); ok {
		if s, ok := c.act.Live.Get(r.Key); ok {
			c.Point(r.Key, s)
		}
	}
}

// toPanel gives the keys to panel p: 1 the tree, 2 the session on the
// right, lit but not entered — Enter goes in, so a number never lands the
// keys in a prompt — 3 the details.
func (c *Chat) toPanel(p int) tea.Cmd {
	switch p {
	case 1:
		c.repFocus = false
		c.ToList()
		return nil
	case 3:
		c.repFocus, c.PaneSel = true, false
		return c.showReport()
	}
	if c.rep.shown {
		c.showChat()
		c.repFocus = false
	}
	if c.tree.OnProject() {
		c.Note("a session takes the keys: this project has none")
		return nil
	}
	c.Choose()
	return nil
}

// enter opens the session under the cursor.
func (c *Chat) enter() {
	if c.Capture.Held() {
		return
	}
	c.PaneSel = false
	if r, ok := c.tree.Session(); ok {
		c.act.Open(r)
	}
}

// selectProjectAt is a click on the n-th heading: the cursor goes there.
// It starts nothing; n does, when a session is wanted.
func (c *Chat) selectProjectAt(n int) {
	if p, ok := c.tree.ProjectAt(n); ok {
		c.tree.SelectProject(p.Name)
	}
}

// selectRow puts the cursor on the n-th session row, as the view numbers
// them for clicks.
func (c *Chat) selectRow(n int) {
	if ss := c.tree.Sessions(); n < len(ss) {
		c.tree.SelectSession(ss[n].Session.Key)
		c.followCursor()
	}
}

// onRow says the cursor is on the n-th session row.
func (c *Chat) onRow(n int) bool {
	s, ok := c.tree.Session()
	ss := c.tree.Sessions()
	return ok && n < len(ss) && ss[n].Session.Key == s.Key
}

// cursorProject names the project of the session under the cursor.
func (c *Chat) cursorProject() string {
	p, _ := c.tree.Project()
	return p.Name
}

// Mouse: the wheel scrolls the session over the pane and the list
// elsewhere, the cursor staying put until a key brings the list back to it; a click on a row puts the cursor there, and on the row already
// under it opens it; a click on a heading jumps to that project; a click on
// a running session's pane gives it the keys, unless the pane shows a
// project, which takes no keys.
func (c *Chat) Mouse(msg tea.MouseMsg) tea.Cmd {
	c.mascot.valid = false
	if kit.LeftClick(msg) {
		// A click is aimed: it goes where it lands, past a selected pane.
		c.PaneSel = false
		if c.tabClick(msg) {
			return nil
		}
	}
	hit := hits.At(msg, len(c.tree.Sessions()), len(c.core.Store.Projects))
	if c.rep.shown {
		if d := kit.Wheel(msg); d != 0 && (hit.Kind == kit.HitPane || hit.Kind == kit.HitPanel && hit.N == 2) {
			c.rep.scroll += d / kit.WheelRows
			return nil
		}
		if kit.LeftClick(msg) && hit.Kind == kit.HitPanel && hit.N == 2 {
			c.repFocus = true
			return nil
		}
		// A click on a session in the tree goes to that session's chat.
		if kit.LeftClick(msg) && hit.Kind == kit.HitRow {
			c.showChat()
			c.repFocus = false
		}
	}
	if d := kit.Wheel(msg); d != 0 {
		if hit.Kind == kit.HitPane {
			c.WheelPane(msg.X, msg.Y, d)
		} else {
			c.list.scroll.Wheel(d / kit.WheelRows)
		}
		return nil
	}
	if c.draftMouse(msg) {
		return nil
	}
	if !kit.LeftClick(msg) {
		return nil
	}
	c.tree.Moving = false // a click puts a picked-up row down where it is
	// A click outside the draft box puts the draft away, kept.
	if c.drafting && !(hit.Kind == kit.HitPanel && hit.N == 3) {
		c.closeDraft()
	}
	switch hit.Kind {
	case kit.HitRow:
		if c.onRow(hit.N) && !c.Capture.Held() {
			c.enter()
			return nil
		}
		c.Capture.Drop()
		c.selectRow(hit.N)
		// A click on a finished session is looking at it: it stops calling.
		if r, ok := c.tree.Current(); ok && r.Session != nil {
			c.board.See(r.Session.Key)
		}
	case kit.HitHeading:
		c.Capture.Drop()
		c.selectProjectAt(hit.N)
	case kit.HitPane:
		if !c.tree.OnProject() {
			c.takeKeys()
		}
	case kit.HitPanel: // the empty part of a box: it takes the keys, the cursor stays
		c.Pane.StopCopy()
		if hit.N == 2 && !c.tree.OnProject() {
			c.takeKeys()
		}
	}
	return nil
}

// pointAt is a click beside a session that had the keys: the cursor goes to
// the row or the project clicked, without opening or starting anything.
func (c *Chat) pointAt(msg tea.MouseMsg) {
	if c.tabClick(msg) {
		return
	}
	hit := hits.At(msg, len(c.tree.Sessions()), len(c.core.Store.Projects))
	switch hit.Kind {
	case kit.HitRow:
		c.selectRow(hit.N)
	case kit.HitHeading:
		c.selectProjectAt(hit.N)
	}
}

// takeKeys gives the shown session the keys, which is looking at it: a
// finished one stops calling at once, and waits quietly for its prompt.
func (c *Chat) takeKeys() {
	if c.Capture.Take() {
		c.board.See(c.Pane.Key)
	}
}

// searchSessions is s on the tree: a finder over every session of every
// project, by name, with its project and tool beside it; the one chosen
// takes the cursor, and the report follows when it shows.
func (c *Chat) searchSessions() {
	rows := c.tree.Sessions()
	if len(rows) == 0 {
		c.Screen.Note("no session yet: n starts one")
		return
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Session.Name
	}
	f := kit.NewFinder("sessions", names, func(i int) {
		c.tree.SelectSession(rows[i].Session.Key)
		c.list.scroll.Follow()
		if cmd := c.reportFollow(); cmd != nil {
			c.Screen.Queue(cmd)
		}
	})
	f.Note = func(i int) string { return rows[i].Project.Name + " · " + rows[i].Session.Tool }
	c.Screen.Push(f)
}
