package git

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/core/syntax"
	"lazychat/internal/ui/git/model"
	"lazychat/internal/ui/kit"
)

// statusMsg is a project's status, read off the loop; diffMsg a change's
// diff, numbered so only the newest one asked for is drawn.
type (
	statusMsg struct {
		name    string // the row's key
		st      git.Status
		err     error
		own     bool // a project's own row, whose worktrees came with it
		wts     []git.Worktree
		linked  bool // the project's folder is a linked worktree
		parted  []git.Entry
		from    string
		commits []git.LogEntry
	}
	diffMsg struct {
		seq   int
		key   string
		files []git.File
		roles [][][]syntax.Span // the files' code colours; nil when off
		err   error
	}
	stagedMsg struct {
		name string
		err  error
	}
	// versionMsg is git's version, asked once, for the section under the list.
	versionMsg struct {
		version string
		err     error
	}
)

// owned is the answer of a read or write this Git started, marked as its
// own: after a workspace switch the old Git's answers reach the new one,
// and a project of the same name there must not take them.
type owned struct {
	by  *Git
	msg tea.Msg
}

// own marks cmd's answer as this Git's.
func (g *Git) own(cmd func() tea.Msg) tea.Cmd {
	return func() tea.Msg { return owned{by: g, msg: cmd()} }
}

func (g *Git) Update(msg tea.Msg) tea.Cmd {
	if o, ok := msg.(owned); ok {
		if o.by != g {
			return nil
		}
		msg = o.msg
	}
	switch msg := msg.(type) {
	case kit.Tick:
		g.tick = msg.N
		if !g.shown {
			return nil
		}
		if !g.primed {
			g.primed = true
			var cmds []tea.Cmd
			for _, p := range g.core.Store.Projects {
				cmds = append(cmds, g.load(p.Name))
			}
			cmds = append(cmds, func() tea.Msg { v, err := git.Version(); return versionMsg{v, err} })
			return tea.Batch(cmds...)
		}
		if msg.N%refreshTicks == 0 {
			return g.loadCursor()
		}
	case branchesMsg:
		g.showBranches(msg)
	case switchedMsg:
		return g.switched(msg)
	case createdMsg:
		return g.created(msg)
	case updatedMsg:
		return g.updated(msg)
	case deletedMsg:
		return msg.then(msg.err)
	case versionMsg:
		g.version, g.versionErr, g.versionRead = msg.version, msg.err, true
	case statusMsg:
		p := g.status[msg.name]
		if p == nil {
			p = &project{}
			g.status[msg.name] = p
		}
		key, lower := "", false
		if r, ok := g.current(); ok && g.isCursor(msg.name) {
			key, lower = r.Key(), r.Lower()
		}
		at, _ := g.cursorRow()
		p.st, p.err, p.loading = msg.st, msg.err, false
		var cmds []tea.Cmd
		if p.again {
			p.again = false
			cmds = append(cmds, g.load(msg.name))
		}
		if msg.own {
			p.wts, p.linked = msg.wts, msg.linked
			// A worktree found is read too; the cursor keeps its row as
			// rows come and go above it.
			for _, w := range msg.wts {
				if k := worktreeKey(msg.name, w.Path); g.status[k] == nil {
					cmds = append(cmds, g.load(k))
				}
			}
			if g.wantPath == "" || !g.selectPath(g.wantPath) {
				g.selectRow(at.key)
			} else {
				g.wantPath = ""
				cmds = append(cmds, g.loadDiff())
			}
		} else {
			p.parted, p.from = msg.parted, msg.from
		}
		p.commits = msg.commits
		if g.isCursor(msg.name) {
			g.keepCursorOn(key, lower)
			cmds = append(cmds, g.loadDiff())
		}
		return tea.Batch(cmds...)
	case committedMsg:
		return g.commitDone(msg)
	case remoteMsg:
		return g.remoteDone(msg)
	case suggestedMsg:
		g.suggested(msg)
	case stagedMsg:
		if p := g.status[msg.name]; p != nil {
			p.writing = false
		}
		if msg.err != nil {
			g.screen.Note("%v", msg.err)
		}
		return g.load(msg.name)
	case diffMsg:
		if msg.seq != g.seq {
			return nil
		}
		next := diff{key: msg.key, title: g.diff.title, lines: flatten(msg.files, msg.roles...), err: msg.err}
		if msg.key == g.diff.key {
			// A refresh of the same change keeps its place and selection.
			next.top, next.cur, next.anchor, next.marked = g.diff.top, g.diff.cur, g.diff.anchor, g.diff.marked
		}
		g.diff = next
		g.diff.cur = kit.Clamp(g.diff.cur, 0, max(0, len(g.diff.lines)-1))
	case kit.WorkspaceMoved:
		g.status, g.primed = map[string]*project{}, false
	}
	return nil
}

func (g *Git) isCursor(key string) bool {
	r, ok := g.cursorRow()
	return ok && r.key == key
}

// keepCursorOn puts the middle cursor back on the node it was on; one that
// went to the other box leaves the cursor where it was, on the next row, as
// Fork does, kept in its own box while that box has rows.
func (g *Git) keepCursorOn(key string, lower bool) {
	nodes := g.nodes()
	last := -1
	for i, n := range nodes {
		if n.Key() == key {
			g.changes.Sel = i
			return
		}
		if n.Lower() == lower && i <= g.changes.Sel {
			last = i
		}
	}
	if last >= 0 && nodes[min(g.changes.Sel, len(nodes)-1)].Lower() != lower {
		g.changes.Sel = last
	}
	g.changes.ClampTo(len(nodes))
}

// toggle stages what is under the cursor in the unstaged box, or unstages
// it in the staged box, off the loop; the status is read again after.
func (g *Git) toggle() tea.Cmd {
	n, ok := g.current()
	p := g.cursorStatus()
	if !ok || p == nil || p.writing {
		return nil
	}
	paths, conflicts := n.Paths()
	if conflicts > 0 {
		g.screen.Note("a conflict is staged once it is resolved: fix it in an editor, then git add it")
	}
	if len(paths) == 0 {
		return nil
	}
	if n.Row.Section == model.Parted || n.Folder && len(n.Rows) > 0 && n.Rows[0].Section == model.Parted {
		g.screen.Note("committed on its branch: nothing to stage")
		return nil
	}
	at, _ := g.cursorRow()
	name, root, lower := at.key, p.st.Root, n.Lower()
	p.writing = true
	return g.own(func() tea.Msg {
		if lower {
			return stagedMsg{name: name, err: git.Unstage(root, paths)}
		}
		return stagedMsg{name: name, err: git.Stage(root, paths)}
	})
}

// load reads a row's status off the loop: a project's with the other
// worktrees of its repository, a worktree's with what its branch changed
// since it parted from the project's.
func (g *Git) load(key string) tea.Cmd {
	var r row
	found := false
	for _, x := range g.rows() {
		if x.key == key {
			r, found = x, true
		}
	}
	if !found {
		return nil
	}
	st := g.status[key]
	if st == nil {
		st = &project{}
		g.status[key] = st
	}
	if st.loading {
		// The read under way may predate what changed (a stage just done),
		// so one more follows it.
		st.again = true
		return nil
	}
	st.loading = true
	dir, wt, base := r.path, r.wt, r.base
	return g.own(func() tea.Msg {
		s, err := git.StatusOf(dir)
		msg := statusMsg{name: key, st: s, err: err, own: wt == nil}
		if err == nil {
			msg.commits, _ = git.Log(dir, commitsRead)
		}
		if err == nil && wt != nil {
			if msg.from = git.StartPoint(s.Root, wt.Branch); msg.from != "" {
				base = msg.from
			}
		}
		switch {
		case err != nil:
		case wt == nil:
			msg.wts, msg.linked, _ = git.OthersAt(s.Root)
		case base != "" && base != "(detached)" && wt.Branch != "" && wt.Branch != base:
			msg.parted, _ = git.Parted(s.Root, base, wt.Branch)
		}
		return msg
	})
}

func (g *Git) loadCursor() tea.Cmd {
	r, ok := g.cursorRow()
	if !ok {
		return nil
	}
	return g.load(r.key)
}

// loadDiff asks for the diff of the change under the middle cursor; with
// none there the right side empties at once.
func (g *Git) loadDiff() tea.Cmd {
	g.seq++
	colour := g.syntaxOn()
	if g.onCommits() {
		return g.loadCommit()
	}
	r, ok := g.current()
	p := g.cursorStatus()
	if !ok || p == nil {
		g.diff = diff{}
		return nil
	}
	seq, key, root := g.seq, r.Key(), p.st.Root
	title := r.Row.Entry.Path
	switch {
	case r.All() && r.Lower():
		title = "all staged"
	case r.All():
		title = "all unstaged"
	case r.Folder:
		title = r.Path
	}
	g.diff.title = title
	if g.diff.key != key {
		g.diff = diff{key: key, title: title}
	}
	var entries []git.Entry
	parted := false
	for _, c := range r.Changes() {
		entries = append(entries, c.Entry)
		parted = c.Section == model.Parted
	}
	staged := r.Lower()
	at, _ := g.cursorRow()
	base, branch := at.base, p.st.Branch
	return g.own(func() tea.Msg {
		var patch string
		var err error
		if parted {
			patch, err = git.PartedDiff(root, base, branch, entries)
		} else {
			patch, err = git.DiffAll(root, entries, staged)
		}
		return g.diffRead(seq, key, patch, err, colour)
	})
}

// diffRead is a read patch as the diff message, its code coloured when
// colour says so; it runs in the read's goroutine.
func (g *Git) diffRead(seq int, key, patch string, err error, colour bool) tea.Msg {
	msg := diffMsg{seq: seq, key: key, files: git.Parse(patch), err: err}
	if colour {
		msg.roles = syntaxOf(msg.files)
	}
	return msg
}

// syntaxOn says a diff's code is coloured: on unless turned off in Settings.
func (g *Git) syntaxOn() bool { return g.core.Settings == nil || !g.core.Settings.NoSyntax }

// moveProject steps the left cursor and reads the new project's changes.
func (g *Git) moveProject(d int) tea.Cmd {
	before := g.projects.Sel
	g.projects.Move(d, len(g.rows()))
	if g.projects.Sel == before {
		return nil
	}
	g.changes.Sel, g.commits.Sel = 0, 0
	return tea.Batch(g.loadCursor(), g.loadDiff())
}

// onCommits says the right side shows a commit: the commits have the keys,
// or had them before the diff or the commit box took them.
func (g *Git) onCommits() bool {
	return g.focus == panelCommits || (g.focus == panelDiff || g.focus == panelCommit) && g.back == panelCommits
}

// loadCommit asks for the patch of the commit under the commits' cursor.
func (g *Git) loadCommit() tea.Cmd {
	p := g.cursorStatus()
	if p == nil || len(p.commits) == 0 {
		g.diff = diff{}
		return nil
	}
	g.commits.ClampTo(len(p.commits))
	c := p.commits[g.commits.Sel]
	seq, key, root, colour := g.seq, "commit "+c.Hash, p.st.Root, g.syntaxOn()
	if g.diff.key != key {
		g.diff = diff{key: key}
	}
	g.diff.title = c.Hash + " " + c.Subject
	return g.own(func() tea.Msg {
		patch, err := git.Show(root, c.Hash)
		return g.diffRead(seq, key, patch, err, colour)
	})
}

func (g *Git) moveCommit(d int) tea.Cmd {
	p := g.cursorStatus()
	if p == nil {
		return nil
	}
	before := g.commits.Sel
	g.commits.Move(d, len(p.commits))
	if g.commits.Sel == before {
		return nil
	}
	return g.loadDiff()
}

func (g *Git) moveChange(d int) tea.Cmd {
	if _, ok := g.current(); !ok && g.changesFocused() {
		return nil // an empty half: nothing to move over
	}
	before := g.changes.Sel
	g.changes.Move(d, len(g.nodes()))
	g.followCursor()
	if g.changes.Sel == before {
		return nil
	}
	return g.loadDiff()
}

func (g *Git) scrollDiff(d int) {
	g.diff.top = kit.Clamp(g.diff.top+d, 0, max(0, len(g.diff.lines)-g.diffRows()))
}

// carry is a step of move mode: the project heading among the projects,
// in Chat and Terminal too, since all three list the state file's projects.
func (g *Git) carry(d int) {
	r, ok := g.cursorRow()
	if !ok {
		return
	}
	if err := g.core.Store.MoveProject(r.name, d); err != nil {
		g.screen.Note("move: %v", err)
		return
	}
	g.selectRow(r.key)
}

// Mouse: a click on a project or a change puts the cursor there; the wheel
// scrolls the diff over it and moves the cursor over a list.
func (g *Git) Mouse(msg tea.MouseMsg) tea.Cmd {
	if kit.LeftClick(msg) && g.boxShown() {
		switch g.box().Click(msg) {
		case kit.CommitNow:
			g.startCommit()
			return g.commit()
		case kit.CommitSuggest:
			g.startCommit()
			return g.suggest()
		case kit.CommitNothing:
			g.startCommit()
			return nil
		}
	}
	commits := 0
	if p := g.cursorStatus(); p != nil {
		commits = len(p.commits)
	}
	if i, ok := hits.ItemAt(msg, commits); ok {
		if d := kit.Wheel(msg); d != 0 {
			return g.moveCommit(d / kit.WheelRows)
		}
		if kit.LeftClick(msg) {
			g.moving, g.focus, g.commits.Sel = false, panelCommits, i
			return g.loadDiff()
		}
		return nil
	}
	if g.diff.dragging {
		return g.dragDiff(msg)
	}
	hit := hits.At(msg, len(g.rows()), len(g.nodes()))
	if d := kit.Wheel(msg); d != 0 {
		switch hit.Kind {
		case kit.HitPane:
			g.scrollDiff(d)
		case kit.HitRow:
			return g.moveProject(d / kit.WheelRows)
		case kit.HitHeading:
			return g.moveChange(d / kit.WheelRows)
		}
		return nil
	}
	if !kit.LeftClick(msg) {
		return nil
	}
	g.moving = false
	switch hit.Kind {
	case kit.HitRow:
		g.focus = panelProjects
		return g.moveProject(hit.N - g.projects.Sel)
	case kit.HitHeading:
		if hit.N-1 < len(g.nodes()) {
			g.focus = panelUnstaged
			g.changes.Sel = hit.N - 1
			g.followCursor()
			return g.loadDiff()
		}
	case kit.HitPane:
		cmd := g.goTo(panelDiff)
		g.pressDiff(msg)
		return cmd
	case kit.HitPanel:
		return g.clickPanel(panel(hit.N))
	}
	return nil
}
