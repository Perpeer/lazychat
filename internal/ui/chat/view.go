package chat

import (
	"fmt"
	"strings"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// narrowWidth is where the two columns stop fitting; below it the terminal
// pane is shown alone on demand.
const narrowWidth = 80

type geometry struct {
	leftW, rightW int
	bodyH         int // the tab's height: the tree's and the pane's
}

func (c *Chat) geometry() geometry {
	g := geometry{bodyH: max(8, c.rect.Rows)}
	g.leftW = kit.ListWidth(c.rect.Cols)
	if c.narrow() {
		g.leftW = c.rect.Cols
	}
	g.rightW = c.rect.Cols - g.leftW
	return g
}

func (c *Chat) narrow() bool { return c.rect.Cols < narrowWidth }

// paneRect is where the terminal pane's inner area is on the screen: its
// size is what the sessions' ptys get. The where line takes the right
// side's top row.
func (c *Chat) paneRect() kit.Rect {
	g := c.geometry()
	if c.narrow() {
		return kit.Rect{X0: c.rect.X0 + 1, Y0: c.rect.Y0 + 1 + whereRows, Cols: c.rect.Cols - 2, Rows: g.bodyH - 2 - whereRows}
	}
	return kit.Rect{X0: c.rect.X0 + g.leftW + 1, Y0: c.rect.Y0 + 1 + whereRows, Cols: g.rightW - 2, Rows: g.bodyH - 2 - whereRows}
}

// whereRows is the line over the right side naming the checkout its
// session works in.
const whereRows = 1

// where is that line for the cursor's project — which repository,
// worktree and branch — so a session's changes are never taken for the
// other checkout's.
func (c *Chat) where(w int) string {
	r, ok := c.tree.Current()
	if !ok || r.Project == nil {
		return text.Pad("", w)
	}
	h, inRepo := c.core.Head(r.Project.Path)
	return text.Pad(kit.WhereLine(h, inRepo, w), w)
}

func (c *Chat) PaneSize() (cols, rows int) {
	r := c.paneRect()
	return r.Cols, r.Rows
}

func (c *Chat) View() string {
	c.list.follow.Sync(&c.core.Selected, c.cursorProject(), func(p string) { c.tree.SelectProject(p) })
	g := c.geometry()
	left := func() string {
		focused := !c.capture.Held()
		rows := kit.WithTools(c.core.ToolStates(), g.leftW-2, g.bodyH-2, func(h int) []string { return c.list.view(g.leftW-2, h, focused) })
		return hits.Panel(1, kit.Box(c.list.title(), rows, g.leftW, g.bodyH, focused, false))
	}
	pane := func(w int) string {
		dh := c.draftH(g.bodyH)
		h := g.bodyH - whereRows
		var right string
		if r, ok := c.tree.Current(); ok && r.Session == nil {
			right = c.projectPanel(r.Project.Name, w, h)
		} else {
			// The block blinks with the tick while the session has the keys; unfocused it is a steady underline.
			term := kit.Box(kit.PanelTitle(2, c.pane.Title()), c.pane.View(w-2, h-2, c.capture.Held(), c.tick%2 == 0), w, h, c.capture.Held(), true)
			from, length := c.pane.Scrollbar(h - 2)
			right = kit.WithScrollbar(term, from, length)
		}
		right = c.where(w) + "\n" + hits.Panel(2, right)
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
	case c.narrow() && c.fullTerm:
		return pane(c.rect.Cols)
	case c.narrow():
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
		return kit.Box(kit.PanelTitle(2, project), append([]string{""}, lines...), w, h, false, false)
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
	return kit.Box(kit.PanelTitle(2, fmt.Sprintf("%s · running (%d)", project, len(running))), append([]string{""}, lines...), w, h, false, false)
}

// Note shows one line in the footer for a few seconds: the result of an action.
func (c *Chat) Note(format string, args ...any) { c.screen.Note(format, args...) }
