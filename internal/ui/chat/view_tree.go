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
	tick     int
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

func (t *treeView) glyph(s state.Session) (styled, plain string) {
	if t.live.Running(s.Key) {
		g := kit.Spinner[(t.tick/2)%len(kit.Spinner)] // the tick is half a second; the spinner turns once a second
		return kit.StyleBusy.Render(g), g
	}
	return kit.StyleDim.Render("○"), "○"
}

// sessionEntry hangs a session off its project's line: ├─ for all but the
// last, └─ for the last, whose rows below then leave the line open.
func (t *treeView) sessionEntry(s state.Session, w int, last, orphan bool) []kit.TreeLine {
	first, rest := "   ├─ ", "   │  "
	if last {
		first, rest = "   └─ ", "      "
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
	return append(out, kit.TreeLine{Prefix: rest, Styled: "  " + kit.ToolBadge(tool) + styled, Plain: "  " + tool + plain})
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
	selRow := -1
	blocks := make([][]string, len(rows))
	heights := make([]int, len(rows))
	project, session := 0, 0
	for i, r := range rows {
		var lead []string // rows above the entry that are not part of it
		var b []string
		switch {
		case r.Empty:
			// Drawn with its heading, inside the heading's click zone.
			if hasCur && cur.Empty && cur.Project.Name == r.Project.Name {
				selRow = i
			}
		case r.Heading():
			if i > 0 {
				lead = []string{""}
			}
			project++
			entry := t.projectEntry(*r.Project, project, w)
			if hasCur && t.tree.Moving && t.tree.Whole && cur.Project != nil && cur.Project.Name == r.Project.Name {
				entry = kit.Picked(entry)
			}
			b = kit.DrawEntry(entry, w, false, focused)
			if i+1 < len(rows) && rows[i+1].Empty {
				empty := hasCur && cur.Empty && cur.Project.Name == r.Project.Name
				b = append(append(b, kit.ChildGap()), kit.DrawEntry(kit.EmptyEntry("no sessions yet"), w, empty, focused)...)
			}
			b = kit.ZoneBlock(fmt.Sprintf("proj-%d", project), b, w)
		default:
			orphan := r.Project == nil
			if orphan && (i == 0 || rows[i-1].Project != nil) {
				lead = []string{"", kit.StyleDim.Render(text.Fit(" ? no longer registered", w))}
			}
			lead = append(lead, kit.ChildGap())
			last := i+1 == len(rows) || rows[i+1].Heading() || rows[i+1].Project != r.Project
			selected := hasCur && cur.Session != nil && r.Session.Key == cur.Session.Key
			if selected {
				selRow = i
			}
			entry := t.sessionEntry(*r.Session, w, last, orphan)
			if selected && t.tree.Moving && !t.tree.Whole {
				entry = kit.Picked(entry)
			}
			b = kit.DrawEntry(entry, w, selected, focused)
			b = kit.ZoneBlock(fmt.Sprintf("row-%d", session), b, w)
			session++
		}
		blocks[i] = append(lead, b...)
		heights[i] = len(blocks[i])
	}
	// The empty row is drawn in its heading's block, so that block is the
	// one to keep in view.
	if selRow > 0 && rows[selRow].Empty {
		selRow--
	}
	headingAbove := selRow > 0 && rows[selRow-1].Heading()
	return kit.DrawBlocks(blocks, t.scroll.Place(heights, selRow, h, headingAbove), h)
}
