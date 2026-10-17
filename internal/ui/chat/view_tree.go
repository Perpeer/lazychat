package chat

import (
	"fmt"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/core/status"
	"lazychat/internal/ui/chat/actions"
	"lazychat/internal/ui/chat/model"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// treeView draws the model's tree: projects as headings, sessions hanging
// off them on connector lines, the running ones spinning.
type treeView struct {
	tree     *model.Tree
	live     *actions.Live
	asking   func(key string) bool                              // the session has a question up; nil for none
	done     func(key string) bool                              // the session finished and was not looked at since
	seen     func(key string) bool                              // the session finished and was looked at, waiting for a prompt
	branch   func(path string) string                           // what the project's folder is on; nil for none
	worktree func(path string) string                           // the worktree a project's folder is, "" for a repository's own; nil for none
	turn     func(key string) (time.Duration, status.TurnState) // the session's turn time; nil for none
	draft    func(r state.Session) bool                         // the session has a draft waiting; nil for none
	working  func(key string) bool                              // the board calls the session at work; nil for none
	tick     int
	beat     int // the fast beat, which sweeps a working session's name
	scroll   kit.Scroller
	follow   kit.Follow
}

// nameLines is how many rows a session's name may wrap to under its
// project's connector line; its tool and turn time are the row after.
const nameLines = 3

func (t *treeView) title() string {
	return kit.PanelTitle(1, "projects")
}

// projectEntry is a project's heading: its name, folder and branch.
func (t *treeView) projectEntry(p state.Project, n, w int) []kit.TreeLine {
	branch := ""
	if t.branch != nil {
		branch = t.branch(p.Path)
	}
	return kit.ProjectHeading(p.Name, p.Path, branch, w)
}

// workFrames turn under a working session's glyph, one a beat.
var workFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// glyph turns only for a session at work: a program that waits on its
// prompt spun too, and the user could not tell which session worked.
func (t *treeView) glyph(s state.Session) (styled, plain string) {
	switch {
	case t.works(s.Key):
		g := kit.Spinner[(t.tick/2)%len(kit.Spinner)] // the tick is half a second; the spinner turns once a second
		return kit.StyleBusy.Render(g), g
	case t.live.Running(s.Key):
		return kit.StyleDim.Render("●"), "●"
	}
	return kit.StyleDim.Render("○"), "○"
}

func (t *treeView) works(key string) bool { return t.working != nil && t.working(key) }

// sessionEntry hangs a session off its project's line: ├─ for all but the
// last, └─ for the last, whose rows below then leave the line open.
func (t *treeView) sessionEntry(s state.Session, w int, last, orphan bool) []kit.TreeLine {
	first, rest := "   ├─ ", "   │  "
	if last {
		first, rest = "   └─ ", "      "
	}
	// A session at work hangs on a heavy line in the busy colour down all its
	// rows, so it is a lit block in the tree: the spinner and the sweep alone
	// were too small to find it by.
	working := t.works(s.Key)
	if working {
		first, rest = "   ┣━ ", "   ┃  "
		if last {
			first = "   ┗━ "
		}
	}
	glyph, plainGlyph := t.glyph(s)
	nameStyle := kit.StyleBold
	// A session with a question up blinks, and one that finished, so they
	// are found in the list.
	// A finished one looked at keeps a steady ✓ until its next prompt.
	if t.seen != nil && t.seen(s.Key) {
		glyph, plainGlyph = kit.StyleBusy.Render("✓"), "✓"
	}
	switch {
	case t.tick%2 != 0:
	case t.asking != nil && t.asking(s.Key):
		glyph, plainGlyph, nameStyle = kit.StyleAccent.Bold(true).Render("?"), "?", kit.StyleAccent.Bold(true)
	case t.done != nil && t.done(s.Key):
		glyph, plainGlyph, nameStyle = kit.StyleBusy.Bold(true).Render("✓"), "✓", kit.StyleBusy.Bold(true)
	}
	out := kit.TitleRows(first, rest, glyph+" ", plainGlyph+" ", s.Name, nameStyle, w-2, nameLines)
	if working {
		out = kit.Sweep(out, glyph+" ", plainGlyph+" ", t.beat)
	}
	// The tool leads the row under the name, in its colour, so sessions of
	// different tools tell apart down the list; the turn time follows.
	tool := s.Tool
	styled, plain := "", ""
	if orphan {
		styled, plain = kit.StyleDim.Render(" · "+s.Project), " · "+s.Project
	}
	if t.turn != nil {
		ts, tp := turnFoot(t.turn(s.Key))
		styled, plain = styled+ts, plain+tp
	}
	// A session in a worktree says so on its own row, so it is not taken
	// for one working in the repository's own folder.
	if t.worktree != nil {
		if p, ok := t.tree.Store.ProjectNamed(s.Project); ok {
			if wt := t.worktree(p.Path); wt != "" {
				styled, plain = styled+kit.StyleWorktree.Render(" · ⑂ "+wt), plain+" · ⑂ "+wt
			}
		}
	}
	if t.draft != nil && t.draft(s) {
		styled, plain = styled+kit.StyleAccent.Render(" ✎"), plain+" ✎"
	}
	// At work, a braille spinner turns in the cells under the glyph, on the
	// fast beat, so the session is found by movement as well as colour.
	lead, plainLead := "  ", "  "
	if working {
		f := workFrames[t.beat%len(workFrames)]
		lead, plainLead = kit.StyleBusy.Render(f)+" ", f+" "
	}
	out = append(out, kit.TreeLine{Prefix: rest, Styled: lead + kit.ToolBadge(tool) + styled, Plain: plainLead + tool + plain})
	for i := range out {
		out[i].Lit = working
	}
	return out
}

// turnFoot is a session's turn time after its tool: counting while it
// works, held in the accent while a question waits, dim once done, and
// nothing before its first prompt.
func turnFoot(d time.Duration, st status.TurnState) (styled, plain string) {
	switch st {
	case status.TurnRunning:
		p := " · ◷ " + text.Span(d)
		return kit.StyleDim.Render(" · ") + "◷ " + text.Span(d), p
	case status.TurnHeld:
		p := " · ⏸ " + text.Span(d)
		return kit.StyleDim.Render(" · ") + kit.StyleAccent.Render("⏸ "+text.Span(d)), p
	case status.TurnDone:
		p := " · " + text.Span(d)
		return kit.StyleDim.Render(p), p
	}
	return "", ""
}

func (t *treeView) view(w, h int, focused bool) []string {
	rows := t.tree.Rows()
	if len(rows) == 0 {
		return []string{kit.StyleDim.Render(text.Fit(" none yet — o opens a project", w))}
	}
	cur, hasCur := t.tree.Current()
	blocks := make([]kit.TreeBlock, len(rows))
	project, session := 0, 0
	for i, r := range rows {
		b := &blocks[i]
		switch {
		case r.Empty:
			// Drawn with its heading, inside the heading's click zone.
			b.Empty, b.Selected = true, hasCur && cur.Empty && cur.Project.Name == r.Project.Name
		case r.Heading():
			b.Heading = true
			if i > 0 {
				b.Lead = []string{""}
			}
			project++
			entry := t.projectEntry(*r.Project, project, w)
			if hasCur && t.tree.Moving && t.tree.Whole && cur.Project != nil && cur.Project.Name == r.Project.Name {
				entry = kit.Picked(entry)
			}
			empty := i+1 < len(rows) && rows[i+1].Empty
			b.Rows = kit.HeadingRows(entry, empty, "no sessions yet", empty && hasCur && cur.Empty && cur.Project.Name == r.Project.Name, w, focused, fmt.Sprintf("proj-%d", project))
		default:
			orphan := r.Project == nil
			if orphan && (i == 0 || rows[i-1].Project != nil) {
				b.Lead = []string{"", kit.StyleDim.Render(text.Fit(" ? no longer registered", w))}
			}
			b.Lead = append(b.Lead, kit.ChildGap())
			last := i+1 == len(rows) || rows[i+1].Heading() || rows[i+1].Project != r.Project
			b.Selected = hasCur && cur.Session != nil && r.Session.Key == cur.Session.Key
			entry := t.sessionEntry(*r.Session, w, last, orphan)
			if b.Selected && t.tree.Moving && !t.tree.Whole {
				entry = kit.Picked(entry)
			}
			b.Rows = kit.ZoneBlock(fmt.Sprintf("row-%d", session), kit.DrawChosen(entry, w, b.Selected), w)
			session++
		}
	}
	return kit.DrawTree(blocks, &t.scroll, h)
}
