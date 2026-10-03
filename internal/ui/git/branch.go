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
		err      error
		fetched  bool
		fetchErr error
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

// branchPopup is the open branch finder and the list it shows, so a list
// that arrives later (the fetch) redraws it in place.
type branchPopup struct {
	project string
	root    string
	finder  *kit.Finder
	list    []git.Branch
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
	bp := &branchPopup{project: name, root: root}
	bp.finder = kit.NewFinder(title, nil, func(i int) { g.pickBranch(bp, i) })
	bp.finder.Group = func(i int) string {
		if bp.list[i].Remote {
			return "Remote"
		}
		return "Local"
	}
	bp.finder.Note = func(i int) string { return branchNote(bp.list[i]) }
	bp.finder.Status = "reading branches…"
	g.branches = bp
	g.screen.Push(bp.finder)
	list := g.own(func() tea.Msg {
		l, err := git.Branches(root)
		return branchesMsg{name: name, list: l, err: err}
	})
	fetch := g.own(func() tea.Msg {
		ferr := git.Fetch(root)
		l, err := git.Branches(root)
		return branchesMsg{name: name, list: l, err: err, fetched: true, fetchErr: ferr}
	})
	return tea.Batch(list, fetch)
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
	bp.list = msg.list
	bp.finder.SetItems(names)
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
	b := bp.list[i]
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
