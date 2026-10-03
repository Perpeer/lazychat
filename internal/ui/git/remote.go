package git

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
)

// remoteMsg is a push, pull or fetch's result for the row with key name.
type remoteMsg struct {
	name, did, branch string
	err               error
}

// remote runs a push, pull or fetch for the row under the cursor, off the
// loop, its branch the row's own (a worktree's for a worktree's row).
func (g *Git) remote(did string, run func(root, branch string) error) tea.Cmd {
	r, ok := g.cursorRow()
	p := g.cursorStatus()
	switch {
	case !ok || p == nil || p.err != nil:
		return nil
	case p.writing:
		g.screen.Note("wait: git is still writing")
		return nil
	case did != "fetch" && p.st.Branch == "(detached)":
		g.screen.Note("detached HEAD: switch to a branch first (b)")
		return nil
	case did == "push" && p.st.Upstream != "" && p.st.Ahead == 0:
		g.screen.Note("nothing to push: %s is level with %s", p.st.Branch, p.st.Upstream)
		return nil
	}
	name, root, branch := r.key, p.st.Root, p.st.Branch
	p.writing = true
	g.screen.Note("%s %s…", map[string]string{"push": "pushing", "pull": "pulling", "fetch": "fetching"}[did], branch)
	return g.own(func() tea.Msg { return remoteMsg{name: name, did: did, branch: branch, err: run(root, branch)} })
}

func (g *Git) push() tea.Cmd {
	return g.remote("push", func(root, b string) error { return git.Push(root, b, false) })
}

func (g *Git) pull() tea.Cmd {
	return g.remote("pull", func(root, _ string) error { return git.Pull(root, false) })
}

func (g *Git) fetch() tea.Cmd {
	return g.remote("fetch", func(root, _ string) error { return git.Fetch(root) })
}

// remoteDone takes a push, pull or fetch's result: a branch with no
// upstream is offered one, a parted branch a rebase, each asked; anything
// else is said on the footer, and the row is read again.
func (g *Git) remoteDone(msg remoteMsg) tea.Cmd {
	p := g.status[msg.name]
	if p != nil {
		p.writing = false
	}
	var none git.ErrNoUpstream
	switch {
	case msg.did == "push" && errors.As(msg.err, &none):
		g.screen.Push(&kit.Confirm{
			Question: fmt.Sprintf("%s tracks no branch yet. Push it to %s and track it there?", msg.branch, none.Remote),
			Yes:      func() { g.screen.Queue(g.again(msg, func(root, b string) error { return git.Push(root, b, true) })) },
		})
		return nil
	case msg.did == "pull" && errors.Is(msg.err, git.ErrDiverged):
		ahead, behind := 0, 0
		if p != nil {
			ahead, behind = p.st.Ahead, p.st.Behind
		}
		g.screen.Push(&kit.Confirm{
			Question: fmt.Sprintf("%s and its upstream have parted (↑%d ↓%d). Rebase your commits onto it, local changes put aside and back?", msg.branch, ahead, behind),
			Yes:      func() { g.screen.Queue(g.again(msg, func(root, _ string) error { return git.Pull(root, true) })) },
		})
		return nil
	case msg.err != nil:
		g.screen.Note("%s %s: %v", msg.did, msg.branch, msg.err)
	case msg.did == "push":
		g.screen.Note("pushed %s", msg.branch)
	case msg.did == "pull":
		g.screen.Note("pulled into %s", msg.branch)
	default:
		g.screen.Note("fetched")
	}
	return tea.Batch(g.load(msg.name), g.loadDiff())
}

// again runs a remote command once more for the row a result came from,
// after the user said yes.
func (g *Git) again(msg remoteMsg, run func(root, branch string) error) tea.Cmd {
	p := g.status[msg.name]
	if p == nil {
		return nil
	}
	p.writing = true
	root := p.st.Root
	return g.own(func() tea.Msg {
		return remoteMsg{name: msg.name, did: msg.did, branch: msg.branch, err: run(root, msg.branch)}
	})
}
