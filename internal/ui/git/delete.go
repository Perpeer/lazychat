package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/core/state"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// Deleting from the b finder, Ctrl+D on its row: a local branch, a remote
// one, or a worktree. Each is asked; what loses work — commits merged
// nowhere here, a worktree's changes — and what reaches others — a remote
// branch — is asked a second time. The finder stays open under the
// questions and reads its list again after.

// deletedMsg is a delete done off the loop; then decides what follows on
// the loop: a second question, a note, an offer.
type deletedMsg struct {
	err  error
	then func(err error) tea.Cmd
}

// deleteOff runs a delete off the loop and hands its error to then.
func (g *Git) deleteOff(op func() error, then func(err error) tea.Cmd) {
	g.screen.Queue(g.own(func() tea.Msg { return deletedMsg{err: op(), then: then} }))
}

// askDelete is Ctrl+D on the finder's row i.
func (g *Git) askDelete(bp *branchPopup, i int) {
	if i < len(bp.wts) {
		g.askRemoveWorktree(bp, bp.wts[i])
		return
	}
	b := bp.list[i-len(bp.wts)]
	if b.Remote {
		remote, name := git.SplitRemote(b.Name)
		g.askRemoteDelete(bp, remote, name)
		return
	}
	g.askBranchDelete(bp, b)
}

// askBranchDelete deletes a local branch, asked; one with commits not
// merged here is asked again, naming them, before it is forced. One that
// tracks a remote branch then offers that one too.
func (g *Git) askBranchDelete(bp *branchPopup, b git.Branch) {
	if b.Current {
		g.screen.Note("%s: %v", b.Name, git.ErrCurrentBranch)
		return
	}
	question := "delete branch " + b.Name + "?"
	if b.Upstream != "" {
		question += " " + b.Upstream + " on the remote stays unless you say so next"
	}
	var done func(force bool) func(err error) tea.Cmd
	done = func(force bool) func(err error) tea.Cmd {
		return func(err error) tea.Cmd {
			var un git.ErrUnmerged
			switch {
			case errors.As(err, &un) && !force:
				g.screen.Push(&kit.Confirm{
					Question: fmt.Sprintf("%s has %d commit(s) not merged here, lost with it: %s. Delete it anyway?", b.Name, len(un.Commits), strings.Join(un.Commits, " · ")),
					Yes:      func() { g.deleteOff(func() error { return git.DeleteBranch(bp.root, b.Name, true) }, done(true)) },
				})
				return nil
			case err != nil:
				g.screen.Note("delete %s: %v", b.Name, err)
				return nil
			}
			g.screen.Note("deleted branch %s", b.Name)
			if b.Upstream != "" {
				remote, name := git.SplitRemote(b.Upstream)
				g.askRemoteDelete(bp, remote, name)
			}
			return g.rereadBranches(bp)
		}
	}
	g.screen.Push(&kit.Confirm{Question: question, Yes: func() {
		g.deleteOff(func() error { return git.DeleteBranch(bp.root, b.Name, false) }, done(false))
	}})
}

// askRemoteDelete deletes a branch on a remote after two questions: the
// second says it goes for everyone who uses that remote.
func (g *Git) askRemoteDelete(bp *branchPopup, remote, name string) {
	full := remote + "/" + name
	g.screen.Push(&kit.Confirm{
		Question: "also delete " + full + " on the remote?",
		Yes: func() {
			g.screen.Push(&kit.Confirm{
				Question: full + " goes from " + remote + " for everyone who uses it, and only a push brings it back. Delete it on the remote?",
				Yes: func() {
					g.deleteOff(func() error { return git.DeleteRemoteBranch(bp.root, remote, name) }, func(err error) tea.Cmd {
						if err != nil {
							g.screen.Note("delete %s: %v", full, err)
							return nil
						}
						g.screen.Note("deleted %s on the remote", full)
						return g.rereadBranches(bp)
					})
				},
			})
		},
	})
}

// askRemoveWorktree removes a worktree, asked: refused while a session of
// its project runs, asked again when it has changes, and its project and
// the project's saved sessions go with it, so its shells close too. Its
// branch is offered after.
func (g *Git) askRemoveWorktree(bp *branchPopup, w git.Worktree) {
	shown := filepath.Base(w.Path)
	p, isProject := g.projectAt(w.Path)
	var records []state.Session
	if isProject {
		for _, r := range g.core.Store.Sessions {
			if r.Project != p.Name {
				continue
			}
			if r.Running {
				g.screen.Note("%s runs %s: close it in Chat first", p.Name, r.Name)
				return
			}
			records = append(records, r)
		}
	}
	question := "remove worktree " + shown + " (" + text.ShortHome(w.Path) + ")? its folder goes"
	if isProject {
		question += fmt.Sprintf(", and the project %s with its %d saved session(s) and its shells", p.Name, len(records))
	}
	var done func(force bool) func(err error) tea.Cmd
	done = func(force bool) func(err error) tea.Cmd {
		return func(err error) tea.Cmd {
			var dirty git.ErrWorktreeDirty
			switch {
			case errors.As(err, &dirty) && !force:
				g.screen.Push(&kit.Confirm{
					Question: shown + " has changes that go with it: " + strings.Join(dirty.Files, ", ") + ". Remove it anyway?",
					Yes:      func() { g.deleteOff(func() error { return git.RemoveWorktree(bp.root, w.Path, true) }, done(true)) },
				})
				return nil
			case err != nil:
				g.screen.Note("remove %s: %v", shown, err)
				return nil
			}
			g.forgetProject(p, isProject, records)
			g.screen.Note("removed worktree %s", shown)
			if w.Branch != "" {
				g.offerBranch(bp, w.Branch)
			}
			return tea.Batch(g.rereadBranches(bp), g.load(bp.owner))
		}
	}
	g.screen.Push(&kit.Confirm{Question: question, Yes: func() {
		g.deleteOff(func() error { return git.RemoveWorktree(bp.root, w.Path, false) }, done(false))
	}})
}

// offerBranch asks whether a removed worktree's branch goes too, as a
// local delete would ask it.
func (g *Git) offerBranch(bp *branchPopup, name string) {
	b := git.Branch{Name: name}
	for _, x := range bp.list {
		if !x.Remote && x.Name == name {
			b = x
		}
	}
	g.askBranchDelete(bp, b)
}

// projectAt is the project whose folder is dir.
func (g *Git) projectAt(dir string) (state.Project, bool) {
	for _, p := range g.core.Store.Projects {
		if p.Path == dir || sameDir(p.Path, dir) {
			return p, true
		}
	}
	return state.Project{}, false
}

// forgetProject drops a removed worktree's project and its session
// records; the Terminal tab stops the project's shells when it sees the
// project gone.
func (g *Git) forgetProject(p state.Project, isProject bool, records []state.Session) {
	if !isProject {
		return
	}
	for _, r := range records {
		if err := g.core.Store.RemoveSession(r.Key); err != nil {
			g.screen.Note("remove %s: %v", r.Name, err)
		}
	}
	if err := g.core.Store.RemoveProject(p.Path); err != nil {
		g.screen.Note("remove %s: %v", p.Name, err)
	}
}

// rereadBranches reads the open finder's list again after a delete.
func (g *Git) rereadBranches(bp *branchPopup) tea.Cmd {
	root, name, base := bp.root, bp.project, bp.base
	return g.own(func() tea.Msg {
		l, err := git.Branches(root)
		wts, _, _ := git.Others(root)
		return branchesMsg{name: name, list: l, wts: wts, suggest: git.FreeName(root, base), err: err, fetched: true}
	})
}
