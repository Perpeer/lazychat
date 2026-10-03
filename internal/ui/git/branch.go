package git

import (
	"errors"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// branchesMsg is a project's branches, read off the loop; after the fetch
// the popup started, fetched is set and fetchErr says how it went.
type (
	branchesMsg struct {
		name     string
		list     []git.Branch
		wts      []git.Worktree // the repository's checkouts but the row's own
		suggest  string         // a free name for another worktree of the row's branch
		err      error
		fetched  bool
		fetchErr error
	}
	// createdMsg is a branch or a worktree made off the loop: dir is a new
	// worktree's folder, "" for a branch.
	createdMsg struct {
		key     string // the row asked from
		project string
		name    string
		from    string
		dir     string
		err     error
	}
	// switchedMsg is a switch done off the loop, stashed when local changes
	// were carried over.
	switchedMsg struct {
		name    string
		root    string
		branch  git.Branch
		stashed bool
		err     error
	}
)

// branchPopup is the open finder and what it lists — the repository's
// other worktrees, then its branches — so a list that arrives later (the
// fetch) redraws it in place. base is the row's branch, what a new branch
// or worktree is made from.
type branchPopup struct {
	project string // the row's key
	owner   string // the project's name
	root    string
	base    string
	suggest string
	finder  *kit.Finder
	wts     []git.Worktree
	list    []git.Branch
}

// taken says a name is a branch already, local or as a remote one a switch
// would make local.
func (bp *branchPopup) taken(name string) bool {
	for _, b := range bp.list {
		if b.Name == name {
			return true
		}
		if _, rest, ok := strings.Cut(b.Name, "/"); ok && b.Remote && rest == name {
			return true
		}
	}
	return false
}

// openBranches is b on a project: a finder over its local and remote
// branches, opened at once with what the repository knows while a fetch
// brings the remotes up to date.
func (g *Git) openBranches() tea.Cmd {
	at, ok := g.cursorRow()
	p := g.cursorStatus()
	if !ok || p == nil {
		return nil
	}
	name := at.key
	shown := at.name
	if at.wt != nil {
		shown += " · " + filepath.Base(at.path)
	}
	if p.err != nil {
		g.screen.Note("%s: %v", shown, p.err)
		return nil
	}
	root := p.st.Root
	title := "switch branch · " + shown
	if p.st.Branch == "(detached)" {
		title += " · detached"
	}
	base := p.st.Branch
	if base == "(detached)" {
		base = "HEAD"
	}
	bp := &branchPopup{project: name, owner: at.name, root: root, base: base}
	bp.finder = kit.NewFinder(title, nil, func(i int) { g.pickBranch(bp, i) })
	bp.finder.Group = func(i int) string {
		switch {
		case i < len(bp.wts):
			return "Worktrees"
		case bp.list[i-len(bp.wts)].Remote:
			return "Remote"
		}
		return "Local"
	}
	bp.finder.Note = func(i int) string {
		if i < len(bp.wts) {
			return "⑂ " + text.ShortHome(bp.wts[i].Path)
		}
		return branchNote(bp.list[i-len(bp.wts)])
	}
	bp.finder.Create = func(q string) []string {
		switch {
		case q == "" && bp.suggest != "":
			return []string{"new worktree " + bp.suggest + " from " + base}
		case q == "" || bp.taken(q):
			return nil
		}
		return []string{"new branch " + q + " from " + base, "new worktree " + q + " from " + base}
	}
	bp.finder.Made = func(i int, q string) {
		g.branches = nil
		if q == "" {
			g.create(bp, bp.suggest, true)
			return
		}
		g.create(bp, q, i == 1)
	}
	bp.finder.Status = "reading branches…"
	g.branches = bp
	g.screen.Push(bp.finder)
	read := func(fetched bool, ferr error) branchesMsg {
		l, err := git.Branches(root)
		wts, _, _ := git.Others(root)
		return branchesMsg{name: name, list: l, wts: wts, suggest: git.FreeName(root, base), err: err, fetched: fetched, fetchErr: ferr}
	}
	list := g.own(func() tea.Msg { return read(false, nil) })
	fetch := g.own(func() tea.Msg { ferr := git.Fetch(root); return read(true, ferr) })
	return tea.Batch(list, fetch)
}

// create makes a branch, switched to, or a worktree on a new branch, off
// the loop, from the row's branch.
func (g *Git) create(bp *branchPopup, name string, worktree bool) {
	root, from := bp.root, bp.base
	msg := createdMsg{key: bp.project, project: bp.owner, name: name, from: from}
	g.screen.Queue(g.own(func() tea.Msg {
		if worktree {
			msg.dir, msg.err = git.AddWorktree(root, name, from)
		} else {
			msg.err = git.CreateBranch(root, name, from)
		}
		return msg
	}))
}

// created takes what create made: a branch is the row's now; a worktree
// is opened as a project of its own, for sessions and shells in it, and
// the cursor goes to its row once the project's worktrees are read again.
func (g *Git) created(msg createdMsg) tea.Cmd {
	switch {
	case msg.err != nil && msg.dir == "":
		g.screen.Note("new %s: %v", msg.name, msg.err)
		return nil
	case msg.dir == "":
		g.screen.Note("made %s from %s and switched to it", msg.name, msg.from)
		g.changes.Sel = 0
		return tea.Batch(g.load(msg.key), g.loadDiff())
	}
	g.wantPath = msg.dir
	shown := filepath.Base(msg.dir)
	if p, err := g.core.Store.AddProject(msg.dir, msg.project+" · "+shown); err != nil {
		g.screen.Note("made worktree %s from %s; as a project: %v", shown, msg.from, err)
	} else {
		g.screen.Note("made worktree %s from %s, opened as %s", shown, msg.from, p.Name)
	}
	return g.load(msg.project)
}

// selectPath puts the left cursor on the row of a folder; false when no
// row has it.
func (g *Git) selectPath(dir string) bool {
	for i, r := range g.rows() {
		if r.path == dir || sameDir(r.path, dir) {
			g.projects.Sel = i
			return true
		}
	}
	return false
}

// sameDir says two folders are one, symlinks resolved (/private/var on
// macOS).
func sameDir(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// branchNote is a branch's dim note: ● for the one checked out, what it
// tracks, how long ago its last commit was.
func branchNote(b git.Branch) string {
	var parts []string
	if b.Current {
		parts = append(parts, "● current")
	}
	if b.Upstream != "" {
		parts = append(parts, b.Upstream)
	}
	if !b.When.IsZero() {
		parts = append(parts, text.Ago(b.When))
	}
	return strings.Join(parts, " · ")
}

// showBranches puts a list in the popup, when it is still the one open.
func (g *Git) showBranches(msg branchesMsg) {
	bp := g.branches
	if bp == nil || bp.project != msg.name {
		return
	}
	if msg.err != nil {
		bp.finder.Status = msg.err.Error()
		return
	}
	names := make([]string, len(msg.list))
	for i, b := range msg.list {
		names[i] = b.Name
	}
	bp.list, bp.wts, bp.suggest = msg.list, msg.wts, msg.suggest
	items := make([]string, 0, len(msg.wts)+len(names))
	for _, w := range msg.wts {
		label := w.Branch
		if label == "" {
			label = "detached " + text.Fit(w.Head, 7)
		}
		items = append(items, label)
	}
	bp.finder.SetItems(append(items, names...))
	switch {
	case !msg.fetched && bp.finder.Status == "reading branches…":
		bp.finder.Status = "fetching the remotes…"
	case msg.fetched && msg.fetchErr != nil:
		bp.finder.Status = msg.fetchErr.Error() + " — the remotes as last fetched"
	case msg.fetched:
		bp.finder.Status = ""
	}
}

// pickBranch is Enter in the popup: the switch runs off the loop, the
// popup closes, and the footer tells how it went.
func (g *Git) pickBranch(bp *branchPopup, i int) {
	g.branches = nil
	if i < len(bp.wts) {
		if !g.selectPath(bp.wts[i].Path) {
			g.screen.Note("%s is not listed here", text.ShortHome(bp.wts[i].Path))
			return
		}
		g.focus = panelProjects
		g.changes.Sel = 0
		g.screen.Queue(tea.Batch(g.loadCursor(), g.loadDiff()))
		return
	}
	b := bp.list[i-len(bp.wts)]
	if b.Current {
		g.screen.Note("already on %s", b.Name)
		return
	}
	g.screen.Queue(g.own(func() tea.Msg {
		return switchedMsg{name: bp.project, root: bp.root, branch: b, err: git.Switch(bp.root, b)}
	}))
}

// switched takes a switch's result: on local changes in the way it asks to
// carry them over; else it says the result and reads the project again.
func (g *Git) switched(msg switchedMsg) tea.Cmd {
	var dirty git.ErrDirty
	switch {
	case errors.As(msg.err, &dirty) && !msg.stashed:
		root, b, name := msg.root, msg.branch, msg.name
		g.screen.Push(&kit.Confirm{
			Question: "Local changes to " + strings.Join(dirty.Files, ", ") + " are in the way of " + b.Name + ". Stash them, switch, and bring them back there?",
			Yes: func() {
				g.screen.Queue(g.own(func() tea.Msg {
					return switchedMsg{name: name, root: root, branch: b, stashed: true, err: git.StashSwitch(root, b)}
				}))
			},
		})
		return nil
	case errors.Is(msg.err, git.ErrPopConflict):
		g.screen.Note("%v", msg.err)
	case msg.err != nil:
		g.screen.Note("switch to %s: %v", msg.branch.Name, msg.err)
		return nil
	case msg.stashed:
		g.screen.Note("switched to %s, your changes with it", localName(msg.branch))
	default:
		g.screen.Note("switched to %s", localName(msg.branch))
	}
	g.changes.Sel = 0
	return tea.Batch(g.load(msg.name), g.loadDiff())
}

// localName is the branch a switch lands on: a remote one's name without
// its remote, since switching makes a local branch tracking it.
func localName(b git.Branch) string {
	if !b.Remote {
		return b.Name
	}
	if _, rest, ok := strings.Cut(b.Name, "/"); ok {
		return rest + " (tracking " + b.Name + ")"
	}
	return b.Name
}
