package terminal

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/ssh"
	"lazychat/internal/core/state"
	"lazychat/internal/ui/kit"
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
	case kit.RunInShell:
		t.runInShell(msg)
	}
	return nil
}

// runInShell runs a command the shell asked for (the update popup) in a
// new shell of the cursor's project, else the first one, and brings this
// tab forward so it is watched.
func (t *Terminal) runInShell(msg kit.RunInShell) {
	p, ok := t.tree.Project()
	if !ok {
		ps := t.core.Store.Projects
		if len(ps) == 0 {
			t.Screen.Note("open a project first: a shell runs in one")
			return
		}
		p = ps[0]
	}
	t.Screen.Switch(t.Name())
	t.act.Run(p, msg.Command)
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
	if r, ok := t.tree.Current(); ok && r.Key() != "" {
		if s, ok := t.act.Live.Get(r.Key()); ok {
			t.Point(r.Key(), s)
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

// enter gives the shell under the cursor the keys; on a connection it
// opens it first when it is not open.
func (t *Terminal) enter() {
	t.PaneSel = false
	if t.Capture.Held() {
		return
	}
	if sh, ok := t.tree.Shell(); ok {
		t.act.Open(asShell(sh))
		return
	}
	if c, ok := t.tree.Conn(); ok {
		if p, ok := t.tree.Project(); ok {
			t.act.Connect(c, p)
		}
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

// shellRows are the rows that are shells or connections, in list order,
// as the view numbers their click zones.
func (t *Terminal) shellRows() []int {
	var out []int
	for i, r := range t.tree.Rows() {
		if r.Key() != "" {
			out = append(out, i)
		}
	}
	return out
}

// newSSH asks for a new connection under the cursor's project, saves it
// and opens it.
func (t *Terminal) newSSH() {
	p, ok := t.tree.Project()
	if !ok {
		return
	}
	t.sshForm("new ssh", state.SSH{Project: p.Name, Auth: ssh.KeyFile}, func(c state.SSH) {
		saved, err := t.core.Store.SaveSSH(c)
		if err != nil {
			t.Note("new ssh: %v", err)
			return
		}
		t.tree.SelectKey(saved.Key)
		t.act.Connect(saved, p)
	})
}

// editSSH changes the connection under the cursor; an open session goes on.
func (t *Terminal) editSSH() {
	c, ok := t.tree.Conn()
	if !ok {
		return
	}
	t.sshForm("edit ssh", c, func(next state.SSH) {
		if _, err := t.core.Store.SaveSSH(next); err != nil {
			t.Note("edit ssh: %v", err)
			return
		}
		t.tree.SelectKey(next.Key)
		t.Note("saved %s", next.Name)
	})
}

// signIns are the form's sign-in choices, in the order shown.
var signIns = []struct{ label, auth string }{{"key file", ssh.KeyFile}, {"agent", ssh.Agent}, {"password", ssh.Password}}

// sshForm is the connection's form, filled from c: a host of the user's ssh
// config, or a host, user and port; how it signs in; the key file, picked
// among the keys in ~/.ssh when there are any. submit gets c changed.
func (t *Terminal) sshForm(title string, c state.SSH, submit func(state.SSH)) {
	hosts := append([]string{"none"}, ssh.ConfigHosts(filepath.Join(ssh.Dir(), "config"))...)
	alias := max(0, slices.Index(hosts, c.Alias))
	auth := max(0, slices.IndexFunc(signIns, func(s struct{ label, auth string }) bool { return s.auth == c.Auth }))
	labels := make([]string, len(signIns))
	for i, s := range signIns {
		labels[i] = s.label
	}
	port := ""
	if c.Port != 0 {
		port = strconv.Itoa(c.Port)
	}
	key := kit.TextField("key file", c.KeyFile)
	if keys := ssh.Keys(ssh.Dir()); len(keys) > 0 {
		if c.KeyFile != "" && !slices.Contains(keys, c.KeyFile) {
			keys = append(keys, c.KeyFile)
		}
		key = kit.ChooserField("key file", keys, max(0, slices.Index(keys, c.KeyFile)))
	}
	fields := []kit.Field{
		kit.ChooserField("from ~/.ssh/config", hosts, alias),
		kit.TextField("name", c.Name),
		kit.TextField("host", c.Host),
		kit.TextField("user", c.User),
		kit.TextField("port (22 when empty)", port),
		kit.ChooserField("sign in", labels, auth),
		key,
	}
	f := kit.NewForm(title, fields, func(v []string) {
		next := c
		next.Alias, next.Host, next.User = "", v[2], v[3]
		if v[0] != "none" {
			next.Alias = v[0]
		}
		next.Name = v[1]
		if next.Name == "" {
			next.Name = cmp.Or(next.Alias, next.Host)
		}
		if next.Alias == "" && next.Host == "" {
			t.Note("%s: a host, or one of ~/.ssh/config's", title)
			return
		}
		next.Port = 0
		if v[4] != "" {
			n, err := strconv.Atoi(v[4])
			if err != nil || n <= 0 || n > 65535 {
				t.Note("%s: the port is a number up to 65535", title)
				return
			}
			next.Port = n
		}
		for _, s := range signIns {
			if s.label == v[5] {
				next.Auth = s.auth
			}
		}
		next.KeyFile = ""
		if next.Auth == ssh.KeyFile {
			next.KeyFile = v[6]
		}
		submit(next)
	})
	t.Screen.Push(&f)
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
