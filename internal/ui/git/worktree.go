package git

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
)

// A worktree is a folder kept for one branch's work; the repository's own is
// for pulling and merging. These are the two places that difference shows
// in what the keys do.

// updatedMsg is a worktree brought up to date with main, off the loop.
type updatedMsg struct {
	key, branch, onto string
	err               error
}

// rowIsWorktree says the cursor's row is an added worktree, not the main
// folder.
func (g *Git) rowIsWorktree() bool {
	at, ok := g.cursorRow()
	if !ok {
		return false
	}
	if at.wt != nil {
		return !at.wt.Main
	}
	p := g.cursorStatus()
	return p != nil && p.linked
}

// updateFromMain is u on a worktree's row: asked, a fetch and a rebase of
// its branch onto the remote's main; a conflict is left for the user.
func (g *Git) updateFromMain() tea.Cmd {
	at, ok := g.cursorRow()
	p := g.cursorStatus()
	switch {
	case !ok || p == nil || p.err != nil || !g.rowIsWorktree():
		return nil
	case p.writing:
		g.screen.Note("wait: git is still writing")
		return nil
	case p.st.Branch == "(detached)":
		g.screen.Note("detached HEAD: this worktree has no branch to bring up to date")
		return nil
	}
	key, root, branch := at.key, p.st.Root, p.st.Branch
	g.screen.Push(&kit.Confirm{
		Question: "Bring " + branch + " up to date with main? lazychat fetches, then replays its commits on the remote's main (git rebase), your local changes put aside and back.",
		Yes: func() {
			if p := g.status[key]; p != nil {
				p.writing = true
			}
			g.screen.Note("updating %s from main…", branch)
			g.screen.Queue(g.own(func() tea.Msg {
				onto, err := git.UpdateFromMain(root)
				return updatedMsg{key: key, branch: branch, onto: onto, err: err}
			}))
		},
	})
	return nil
}

// updated says how the update went and reads the row again.
func (g *Git) updated(msg updatedMsg) tea.Cmd {
	if p := g.status[msg.key]; p != nil {
		p.writing = false
	}
	var conflict git.ErrRebaseConflict
	switch {
	case errors.As(msg.err, &conflict):
		g.screen.Note("%v", msg.err)
	case msg.err != nil:
		g.screen.Note("update: %v", msg.err)
	default:
		g.screen.Note("%s is up to date with %s", msg.branch, msg.onto)
	}
	return tea.Batch(g.load(msg.key), g.loadDiff())
}

// onMainInMainFolder says a commit on the cursor's row would land on main
// in the repository's own folder, kept for pulling and merging.
func (g *Git) onMainInMainFolder() bool {
	p := g.cursorStatus()
	if p == nil || g.rowIsWorktree() {
		return false
	}
	return p.st.Branch == "main" || p.st.Branch == "master"
}
