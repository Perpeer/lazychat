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

// Deleting is d on a row: a worktree's row removes the worktree, the
// project's own row deletes the branch it is on, after switching to the
// default branch, since git deletes no branch a checkout is on. Each is
// asked, with its own warning; what loses work — commits not merged, a
// worktree's changes — and what reaches others — a remote branch — is
// asked a second time. b and w only make.

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

// deleteRow is d on the cursor's row.
func (g *Git) deleteRow() {
	at, ok := g.cursorRow()
	p := g.cursorStatus()
	if !ok || p == nil {
		return
	}
	if p.err != nil {
		g.screen.Note("%s: %v", at.name, p.err)
		return
	}
	if at.wt != nil {
		g.askRemoveWorktree(at, p.st.Root)
		return
	}
	// Read now: the row's status may still be the one before a switch.
	g.askCheckoutDelete(at, p.st.Root, git.CurrentBranch(p.st.Root))
}

// askCheckoutDelete deletes the branch the project's own checkout is on:
// the checkout switches to the default branch first, which itself is never
// deleted. Commits not merged there are asked again, naming them; a branch
// that tracks a remote one offers that one after.
func (g *Git) askCheckoutDelete(at row, root, branch string) {
	if branch == "" {
		g.screen.Note("%s is on no branch: nothing to delete", at.name)
		return
	}
	def, err := git.DefaultBranch(root)
	if err != nil {
		g.screen.Note("delete %s: %v", branch, err)
		return
	}
	if branch == def {
		g.screen.Note("%s is the default branch: it is not deleted", branch)
		return
	}
	upstream := git.Upstream(root, branch)
	question := "delete branch " + branch + "? the checkout switches to " + def + " first"
	if upstream != "" {
		question += "; " + upstream + " on the remote stays unless you say so next"
	}
	var done func(force bool) func(err error) tea.Cmd
	done = func(force bool) func(err error) tea.Cmd {
		return func(err error) tea.Cmd {
			var un git.ErrUnmerged
			var dirty git.ErrDirty
			switch {
			case errors.As(err, &dirty):
				g.screen.Note("local changes are in the way of %s: commit them, or switch with b first", def)
				return nil
			case errors.As(err, &un) && !force:
				g.screen.Push(&kit.Confirm{
					Question: fmt.Sprintf("%s has %d commit(s) not in %s, lost with it: %s. Delete it anyway? (the checkout is on %s now)", branch, len(un.Commits), def, strings.Join(un.Commits, " · "), def),
					Yes:      func() { g.deleteOff(func() error { return git.DeleteBranch(root, branch, true) }, done(true)) },
				})
				return g.load(at.key)
			case err != nil:
				g.screen.Note("delete %s: %v", branch, err)
				return g.load(at.key)
			}
			g.screen.Note("switched to %s and deleted branch %s", def, branch)
			if upstream != "" {
				remote, name := git.SplitRemote(upstream)
				g.askRemoteDelete(at, root, remote, name)
			}
			return g.load(at.key)
		}
	}
	g.screen.Push(&kit.Confirm{Question: question, Yes: func() {
		g.deleteOff(func() error {
			if err := git.Switch(root, git.Branch{Name: def}); err != nil {
				return err
			}
			return git.DeleteBranch(root, branch, false)
		}, done(false))
	}})
}

// askRemoteDelete deletes a branch on a remote after two questions: the
// second says it goes for everyone who uses that remote.
func (g *Git) askRemoteDelete(at row, root, remote, name string) {
	full := remote + "/" + name
	g.screen.Push(&kit.Confirm{
		Question: "also delete " + full + " on the remote?",
		Yes: func() {
			g.screen.Push(&kit.Confirm{
				Question: full + " goes from " + remote + " for everyone who uses it, and only a push brings it back. Delete it on the remote?",
				Yes: func() {
					g.deleteOff(func() error { return git.DeleteRemoteBranch(root, remote, name) }, func(err error) tea.Cmd {
						if err != nil {
							g.screen.Note("delete %s: %v", full, err)
							return nil
						}
						g.screen.Note("deleted %s on the remote", full)
						return g.load(at.key)
					})
				},
			})
		},
	})
}

// askRemoveWorktree removes the row's worktree, asked: refused while a
// session of its project runs, asked again when it has changes, and its
// project and the project's saved sessions go with it, so its shells close
// too. Its branch stays.
func (g *Git) askRemoveWorktree(at row, root string) {
	w := *at.wt
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
	if w.Branch != "" {
		question += "; its branch " + w.Branch + " stays"
	}
	var done func(force bool) func(err error) tea.Cmd
	done = func(force bool) func(err error) tea.Cmd {
		return func(err error) tea.Cmd {
			var dirty git.ErrWorktreeDirty
			switch {
			case errors.As(err, &dirty) && !force:
				g.screen.Push(&kit.Confirm{
					Question: shown + " has changes that go with it: " + strings.Join(dirty.Files, ", ") + ". Remove it anyway?",
					Yes:      func() { g.deleteOff(func() error { return git.RemoveWorktree(root, w.Path, true) }, done(true)) },
				})
				return nil
			case err != nil:
				g.screen.Note("remove %s: %v", shown, err)
				return nil
			}
			g.forgetProject(p, isProject, records)
			g.screen.Note("removed worktree %s", shown)
			return g.load(at.name)
		}
	}
	g.screen.Push(&kit.Confirm{Question: question, Yes: func() {
		g.deleteOff(func() error { return git.RemoveWorktree(root, w.Path, false) }, done(false))
	}})
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
