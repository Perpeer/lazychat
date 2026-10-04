// Package git is the Git tab: the same projects as Chat and Terminal on the
// left, the changes of the one under the cursor in the middle — conflicts,
// unstaged over staged, each a folder tree — and the diff of the change
// under the middle cursor on the right, as Fork shows it. Space stages or
// unstages what is under the cursor; everything else only reads, off the
// screen's loop, and takes no lock.
package git

import (
	"time"

	"lazychat/internal/core/api"
	"lazychat/internal/core/git"
	"lazychat/internal/ui/git/model"
	"lazychat/internal/ui/kit"
)

// refreshTicks is how often the cursor's project is read again while the
// tab is on screen: 6 ticks of half a second.
const refreshTicks = 6

// project is what was last read of one row: a project's own checkout or
// one of its repository's other worktrees.
type project struct {
	st      git.Status
	err     error
	loading bool
	again   bool // a read was asked while one ran: read again once it is in
	writing bool // a stage or unstage runs; another waits for its end
	// wts is, on a project's own row, its repository's other worktrees;
	// parted is, on a worktree's, what its branch changed since it parted
	// from the project's.
	wts    []git.Worktree
	linked bool // on a project's own row: its folder is a linked worktree
	parted []git.Entry
	// from is, on a worktree's row, the branch git noted its branch was
	// made from; commits are the checkout's last ones.
	from    string
	commits []git.LogEntry
}

// commitsRead is how many commits a row reads; the box shows what fits.
const commitsRead = 30

// row is one stop of the projects column's cursor: a project's branch, or
// one of the worktrees under it.
type row struct {
	key   string // its status's and commit box's: the project's name, or that and the worktree's path
	name  string // the project's
	path  string // the folder git reads
	wt    *git.Worktree
	base  string // on a worktree's row, the branch it is compared with: the one it was made from, else the project's
	index int    // the project's, in the store
}

func worktreeKey(name, path string) string { return name + "\x00" + path }

// rows is every stop of the left cursor: each project's branch, then the
// worktrees its repository has besides it.
func (g *Git) rows() []row {
	var out []row
	for i, p := range g.core.Store.Projects {
		out = append(out, row{key: p.Name, name: p.Name, path: p.Path, index: i})
		st := g.status[p.Name]
		if st == nil {
			continue
		}
		for j := range st.wts {
			w := st.wts[j]
			key, base := worktreeKey(p.Name, w.Path), st.st.Branch
			if ws := g.status[key]; ws != nil && ws.from != "" {
				base = ws.from
			}
			out = append(out, row{key: key, name: p.Name, path: w.Path, wt: &w, base: base, index: i})
		}
	}
	return out
}

// cursorRow is the row under the left cursor.
func (g *Git) cursorRow() (row, bool) {
	rs := g.rows()
	if len(rs) == 0 {
		return row{}, false
	}
	g.projects.ClampTo(len(rs))
	return rs[g.projects.Sel], true
}

// selectRow puts the left cursor on the row with key, where it is.
func (g *Git) selectRow(key string) {
	for i, r := range g.rows() {
		if r.key == key {
			g.projects.Sel = i
			return
		}
	}
}

// diff is the diff drawn on the right, for the change whose key it holds.
type diff struct {
	key   string
	title string
	lines []line
	err   error
	top   int
	// cur is the diff's row cursor and anchor, when marked, the other end
	// of a selection; dragging is a mouse selection under way.
	cur, anchor      int
	marked, dragging bool
}

type Git struct {
	core   *api.Core
	screen kit.Screen
	rect   kit.Rect

	projects kit.List // the cursor over the projects
	changes  kit.List // the cursor over the selectable changes
	commits  kit.List // the cursor over the row's last commits
	// commitSel is the commit box chosen by its number: lit, typed into
	// only after Enter.
	commitSel bool
	focus     panel // what has the keys
	back      panel // the list the diff or the commit box took the keys from
	moving    bool  // a project is picked up (m)
	tick      int
	shown     bool // on screen since the last Blur
	primed    bool // every project's status was asked for once

	branches *branchPopup // the branch finder while it is open
	wantPath string       // a folder whose row takes the cursor once it is listed
	// mainCommitOK is a yes to committing on main in the main folder, for
	// the commit it was asked for.
	mainCommitOK bool

	// git's own version, for the section under the projects.
	version     string
	versionErr  error
	versionRead bool

	status  map[string]*project       // by project name
	boxes   map[string]*kit.CommitBox // by project name
	diff    diff
	seq     int // the newest diff asked for; an older answer is dropped
	listTop kit.Scroller
}

var _ kit.Tab = (*Git)(nil)

func New(core *api.Core, screen kit.Screen) *Git {
	return &Git{core: core, screen: screen, focus: panelProjects, back: panelProjects, status: map[string]*project{}, boxes: map[string]*kit.CommitBox{}}
}

func (g *Git) Name() string       { return "git" }
func (g *Git) Resize(r kit.Rect)  { g.rect = r }
func (g *Git) Typing() bool       { return g.focus == panelCommit && !g.commitSel }
func (g *Git) Running() int       { return 0 }
func (g *Git) Stop(time.Duration) {}
func (g *Git) Blur() {
	g.shown, g.moving = false, false
	if g.focus == panelCommit {
		g.leave()
	}
}

func (g *Git) Status() string {
	if p := g.cursorStatus(); p != nil && p.loading {
		return "reading git…"
	}
	return ""
}

// cursorStatus is what is known of the row under the cursor.
func (g *Git) cursorStatus() *project {
	r, ok := g.cursorRow()
	if !ok {
		return nil
	}
	return g.status[r.key]
}

// nodes are the changes of the project under the cursor as the middle
// column draws them: the unstaged box's tree, then the staged box's.
func (g *Git) nodes() []model.Node {
	p := g.cursorStatus()
	if p == nil || p.err != nil {
		return nil
	}
	var upper, lower, parted []model.Row
	for _, r := range model.Rows(p.st) {
		if r.Section.Lower() {
			lower = append(lower, r)
		} else {
			upper = append(upper, r)
		}
	}
	for _, e := range p.parted {
		parted = append(parted, model.Row{Section: model.Parted, Entry: e})
	}
	// What a worktree's branch changed comes after its staged changes, a
	// tree of its own, so its folders do not mix with theirs.
	return append(append(model.WithAll(model.Tree(upper)), model.WithAll(model.Tree(lower))...), model.WithAll(model.Tree(parted))...)
}

// current is the file or folder under the middle cursor; none while the
// keys are on a half that is empty.
func (g *Git) current() (model.Node, bool) {
	nodes := g.nodes()
	if len(nodes) == 0 {
		return model.Node{}, false
	}
	g.changes.ClampTo(len(nodes))
	n := nodes[g.changes.Sel]
	if g.changesFocused() && n.Lower() != (g.focus == panelStaged) {
		return model.Node{}, false
	}
	return n, true
}

// followCursor gives the keys to the half the cursor is in, as it moves
// from one box into the other.
func (g *Git) followCursor() {
	nodes := g.nodes()
	if !g.changesFocused() || len(nodes) == 0 {
		return
	}
	g.changes.ClampTo(len(nodes))
	if nodes[g.changes.Sel].Lower() {
		g.focus = panelStaged
	} else {
		g.focus = panelUnstaged
	}
}
