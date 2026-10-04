package terminal

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/terminal/model"
)

// hits are the zones the view marks: shells as rows, projects as headings.
var hits = kit.Hits{Row: "trow", Heading: "tproj", Pane: "tterm", Panels: []string{"tpanel-1", "tpanel-2"}}

func (t *Terminal) Update(msg tea.Msg) tea.Cmd {
	if t.Capture.Update(msg) {
		return nil
	}
	switch msg := msg.(type) {
	case kit.Tick:
		t.Tick = msg.N
		if msg.N%pruneTicks == 0 {
			t.act.Prune()
		}
	case termMsg:
		t.act.Live.AckAll()
		t.act.Reap()
	}
	return nil
}

func (t *Terminal) Key(msg tea.KeyMsg) tea.Cmd {
	t.scroll.Follow()
	return kit.Dispatch(t.bindings(), msg.String(), t)
}

func (t *Terminal) move(d int) {
	if t.Capture.Held() {
		return
	}
	t.tree.Step(d)
	t.follow()
}

// follow makes the pane show the shell under the cursor, so browsing the
// list switches terminals.
func (t *Terminal) follow() {
	if sh, ok := t.tree.Shell(); ok {
		if s, ok := t.act.Live.Get(sh.Key); ok {
			t.Point(sh.Key, s)
		}
	}
}

// carry is a step of move mode: a shell among its project's, or with M
// the cursor's project among the projects.
func (t *Terminal) carry(d int) {
	var err error
	if t.tree.Whole {
		err = t.tree.MoveProject(d)
	} else {
		err = t.tree.MoveShell(d)
	}
	if err != nil {
		t.Screen.Note("move: %v", err)
	}
}

// enter gives the shell under the cursor the keys.
func (t *Terminal) enter() {
	t.PaneSel = false
	if sh, ok := t.tree.Shell(); ok && !t.Capture.Held() {
		t.act.Open(asShell(sh))
	}
}

func (t *Terminal) newShell() {
	if p, ok := t.tree.Project(); ok {
		t.act.Start(p)
	}
}

// toPanel gives the keys to panel p: 1 the list, 2 the shell on the right.
func (t *Terminal) toPanel(p int) {
	if p == 1 {
		t.ToList()
		return
	}
	if t.tree.OnProject() {
		t.Note("a terminal takes the keys: this project has none")
		return
	}
	t.Choose()
}

// shellRows are the rows that are shells, in list order, as the view
// numbers their click zones.
func (t *Terminal) shellRows() []int {
	var out []int
	for i, r := range t.tree.Rows() {
		if r.Shell != nil {
			out = append(out, i)
		}
	}
	return out
}

func (t *Terminal) selectShell(n int) {
	if rows := t.shellRows(); n >= 0 && n < len(rows) {
		t.tree.Sel = rows[n]
		t.follow()
	}
}

func (t *Terminal) selectProject(n int) {
	ps := t.core.Store.Projects
	if n >= 1 && n <= len(ps) {
		t.tree.SelectProject(ps[n-1].Name)
	}
}

// Mouse: the wheel scrolls the shell over the pane and the list elsewhere; a
// click on a row puts the cursor there, and on the row already under it
// opens it; a click on the pane gives the shown shell the keys, and a drag
// there selects its text, copied on release.
func (t *Terminal) Mouse(msg tea.MouseMsg) tea.Cmd {
	hit := hits.At(msg, len(t.shellRows()), len(t.core.Store.Projects))
	if d := kit.Wheel(msg); d != 0 {
		if hit.Kind == kit.HitPane {
			t.WheelPane(msg.X, msg.Y, d)
		} else {
			t.scroll.Wheel(d / kit.WheelRows)
		}
		return nil
	}
	if !kit.LeftClick(msg) {
		return nil
	}
	t.tree.Moving, t.PaneSel = false, false
	switch hit.Kind {
	case kit.HitRow:
		if rows := t.shellRows(); hit.N < len(rows) && rows[hit.N] == t.tree.Sel && !t.Capture.Held() {
			t.enter()
			return nil
		}
		t.Capture.Drop()
		t.selectShell(hit.N)
	case kit.HitHeading:
		t.Capture.Drop()
		t.selectProject(hit.N)
	case kit.HitPane, kit.HitPanel:
		if hit.Kind == kit.HitPanel && hit.N != 2 {
			return nil
		}
		t.Pane.StopCopy()
		if !t.tree.OnProject() && t.Pane.Session != nil {
			t.Capture.Take()
			// The click that takes the keys also starts a selection: its
			// drag and release arrive as the shell's raw mouse after it.
			if r := t.PaneRect(); hit.Kind == kit.HitPane {
				t.Pane.Press(r, msg.X-r.X0, msg.Y-r.Y0)
			}
		}
	}
	return nil
}

// pointAt is a click beside a shell that had the keys: the cursor goes to
// the row or the project clicked, without opening anything.
func (t *Terminal) pointAt(msg tea.MouseMsg) {
	hit := hits.At(msg, len(t.shellRows()), len(t.core.Store.Projects))
	switch hit.Kind {
	case kit.HitRow:
		t.selectShell(hit.N)
	case kit.HitHeading:
		t.selectProject(hit.N)
	}
}

// search is s on the list: a finder over every shell of every project by
// name, its project beside it; the one chosen takes the cursor and the
// pane follows.
func (t *Terminal) search() {
	var rows []model.Row
	for _, r := range t.tree.Rows() {
		if r.Shell != nil {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		t.Screen.Note("no shell yet: n opens one")
		return
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Shell.Name
	}
	f := kit.NewFinder("shells", names, func(i int) {
		t.tree.SelectKey(rows[i].Shell.Key)
		t.follow()
	})
	f.Note = func(i int) string { return rows[i].Project.Name }
	t.Screen.Push(f)
}
