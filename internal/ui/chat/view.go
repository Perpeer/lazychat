package chat

import (
	"strings"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

type geometry struct {
	leftW, rightW int
	bodyH         int // the tab's height: the tree's and the pane's
}

func (c *Chat) geometry() geometry {
	g := geometry{bodyH: c.BodyH(), leftW: c.LeftW()}
	g.rightW = c.Rect.Cols - g.leftW
	return g
}

func (c *Chat) View() string {
	c.mascot.valid = false // what a frame shows is never older than the last one
	c.list.follow.Sync(&c.core.Selected, c.cursorProject(), func(p string) { c.tree.SelectProject(p) })
	g := c.geometry()
	left := func() string {
		focused := !c.Capture.Held() && !(c.rep.shown && c.repFocus) && !c.PaneSel
		rows := kit.WithTools(c.core.ToolStates(), g.leftW-2, g.bodyH-2, func(h int) []string { return c.list.view(g.leftW-2, h, focused) })
		return hits.Panel(1, kit.Box(c.list.title(), rows, g.leftW, g.bodyH, focused, false))
	}
	pane := func(w int) string {
		dh := c.draftH(g.bodyH)
		h := g.bodyH
		var right string
		r, ok := c.tree.Current()
		switch {
		case c.rep.shown:
			right = c.reportBox(w, h)
		case ok && r.Session == nil:
			right = c.projectPanel(r.Project.Name, w, h)
		default:
			// The block blinks with the tick while the session has the keys; unfocused it is a steady underline.
			term := kit.Box(c.chatTabs(c.Pane.State()), c.Pane.View(w-2, h-2, c.Capture.Held(), c.Tick%2 == 0), w, h, c.Capture.Held() || c.PaneSel, true)
			from, length := c.Pane.Scrollbar(h - 2)
			right = kit.WithScrollbar(term, from, length)
		}
		right = hits.Panel(2, right)
		if dh > 0 {
			// Over the pane's lower rows, so the session keeps its size.
			rows := strings.Split(right, "\n")
			if cut := len(rows) - dh; cut >= 0 {
				right = strings.Join(append(rows[:cut], c.draftView(w, dh)), "\n")
			}
		}
		return right
	}
	switch {
	case c.Narrow() && c.FullTerm:
		return pane(c.Rect.Cols)
	case c.Narrow():
		return left()
	default:
		return kit.JoinHorizontal(left(), pane(g.rightW))
	}
}

// projectPanel is the right side while the cursor is on a project: its
// running sessions, only to look at, or with none one sentence on what the
// keys can do there. Nothing here takes the keys.
func (c *Chat) projectPanel(project string, w, h int) string {
	running := c.tree.Running(project, c.act.Live.Running)
	var lines []string
	if len(running) == 0 {
		hint := "Nothing runs in " + project + ": (n) new session, (r) resume session"
		if len(c.tree.Running(project, func(string) bool { return true })) > 0 {
			hint += ", ↓ goes to its sessions"
		}
		for _, l := range text.Wrap(hint+".", w-4, "") {
			lines = append(lines, kit.StyleDim.Render(" "+l))
		}
		return kit.Box(c.chatTabs(""), append([]string{""}, lines...), w, h, false, false)
	}
	for _, s := range running {
		glyph, _ := c.list.glyph(s)
		tool := s.Tool
		for i, n := range text.WrapTitle(s.Name, w-6, 2) {
			lead := "   "
			if i == 0 {
				lead = " " + glyph + " "
			}
			lines = append(lines, lead+kit.StyleBold.Render(n))
		}
		foot, _ := turnFoot(c.turnTime(s.Key))
		lines = append(lines, "   "+kit.ToolBadge(tool)+foot, "")
	}
	return kit.Box(c.chatTabs(""), append([]string{""}, lines...), w, h, false, false)
}
