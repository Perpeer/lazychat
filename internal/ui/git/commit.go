package git

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
)

// committedMsg is a commit's end.
type committedMsg struct {
	name string
	err  error
}

// minCommitRows is the tab height from which the commit box stands under
// the diff all the time; below it the box shows only while it has the keys.
const minCommitRows = 20

// box is the commit box of the project under the cursor; what is typed stays
// with its project while lazychat runs.
func (g *Git) box() *kit.CommitBox {
	at, ok := g.cursorRow()
	if !ok {
		return nil
	}
	name := at.key
	if g.boxes[name] == nil {
		g.boxes[name] = kit.NewCommitBox("gcommit")
	}
	return g.boxes[name]
}

// boxShown is whether the commit box takes the bottom of the right column.
func (g *Git) boxShown() bool {
	p := g.cursorStatus()
	return g.box() != nil && p != nil && p.err == nil && (g.bodyH() >= minCommitRows || g.focus == panelCommit)
}

// staged is how many paths the index holds for the cursor's project.
func (g *Git) staged() int {
	p := g.cursorStatus()
	if p == nil || p.err != nil {
		return 0
	}
	n := 0
	for _, e := range p.st.Entries {
		if e.Conflict == "" && !e.Untracked && e.Staged != '.' {
			n++
		}
	}
	return n
}

// startCommit gives the commit box the keys, on its subject.
func (g *Git) startCommit() {
	if b := g.box(); b != nil && g.cursorStatus() != nil && g.cursorStatus().err == nil {
		if g.focus != panelCommit && g.focus != panelDiff {
			g.back = g.focus
		}
		g.focus, g.commitSel = panelCommit, false
		b.Focus(kit.CommitSubject)
	}
}

// commitKey is a key while the box has them: the box's own first, then the
// few the tab keeps (Esc).
func (g *Git) commitKey(msg tea.KeyMsg) tea.Cmd {
	switch g.box().Key(msg) {
	case kit.CommitNow:
		return g.commit()
	case kit.CommitSuggest:
		return g.suggest()
	case kit.CommitNotMine:
		return kit.Dispatch(commitKeys, msg.String(), g)
	}
	return nil
}

// commit records the staged changes with the box's text, off the loop.
func (g *Git) commit() tea.Cmd {
	b, p := g.box(), g.cursorStatus()
	if b == nil || p == nil || p.writing {
		return nil
	}
	if !b.Enabled(g.staged()) {
		if b.Subject() == "" {
			g.screen.Note("a commit needs a subject")
		} else {
			g.screen.Note("nothing staged: space stages a file or folder")
		}
		return nil
	}
	if g.onMainInMainFolder() && !g.mainCommitOK {
		branch := p.st.Branch
		g.screen.Push(&kit.Confirm{
			Question: "Commit on " + branch + " in the repository itself? Its folder is kept for pulling and merging; work goes in a worktree (w).",
			Yes: func() {
				g.mainCommitOK = true
				g.screen.Queue(g.commit())
			},
		})
		return nil
	}
	g.mainCommitOK = false
	at, _ := g.cursorRow()
	name, root := at.key, p.st.Root
	subject, body := b.Subject(), b.Description()
	p.writing = true
	return g.own(func() tea.Msg {
		return committedMsg{name: name, err: git.Commit(root, subject, body)}
	})
}

// commitDone takes a commit's end: the box empties and the keys go back to
// the changes; on an error the text stays and the footer says why.
func (g *Git) commitDone(msg committedMsg) tea.Cmd {
	if p := g.status[msg.name]; p != nil {
		p.writing = false
	}
	if msg.err != nil {
		g.screen.Note("commit: %v", msg.err)
		return nil
	}
	if b := g.boxes[msg.name]; b != nil {
		b.Clear()
	}
	if g.focus == panelCommit {
		g.leave()
	}
	g.screen.Note("committed")
	return g.load(msg.name)
}

// suggestedMsg is a tool's commit message for a row's staged changes.
type suggestedMsg struct {
	key, subject, body, tool string
	err                      error
}

// suggest asks the AI tool Settings names for a message for what is staged
// on the cursor's row, off the loop; text already typed is replaced only
// once the user says so.
func (g *Git) suggest() tea.Cmd {
	b, p := g.box(), g.cursorStatus()
	if b == nil || p == nil || p.err != nil || b.Suggesting {
		return nil
	}
	if g.staged() == 0 {
		g.screen.Note("nothing staged: space stages what the message is to be about")
		return nil
	}
	at, _ := g.cursorRow()
	key, root := at.key, p.st.Root
	ask := func() tea.Cmd {
		b.Suggesting = true
		return g.own(func() tea.Msg {
			subject, body, tool, err := g.core.SuggestCommit(context.Background(), root)
			return suggestedMsg{key: key, subject: subject, body: body, tool: tool, err: err}
		})
	}
	if b.Subject() == "" && b.Description() == "" {
		return ask()
	}
	g.screen.Push(&kit.Confirm{Question: "replace what is typed with a suggestion?", Yes: func() { g.screen.Queue(ask()) }})
	return nil
}

// suggested fills the row's box with the tool's message, or says why not.
func (g *Git) suggested(msg suggestedMsg) {
	b := g.boxes[msg.key]
	if b == nil {
		return
	}
	b.Suggesting = false
	if msg.err != nil {
		g.screen.Note("suggest: %v", msg.err)
		return
	}
	b.SetMessage(msg.subject, msg.body)
	g.screen.Note("%s suggested the message: edit it, then ctrl+s", msg.tool)
}
