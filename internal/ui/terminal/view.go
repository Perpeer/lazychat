package terminal

import (
	"fmt"
	"path/filepath"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/terminal/actions"
	"lazychat/internal/ui/text"
)

// narrowWidth is where the two columns stop fitting; below it the shell is
// shown alone while it has the keys.
const narrowWidth = 80

// nameLines is how many rows a shell's name may wrap to.
const nameLines = 2

func (t *Terminal) narrow() bool { return t.rect.Cols < narrowWidth }

func (t *Terminal) leftW() int {
	if t.narrow() {
		return t.rect.Cols
	}
	return kit.ListWidth(t.rect.Cols)
}

func (t *Terminal) bodyH() int { return max(8, t.rect.Rows) }

// paneRect is where the shell's inner area is on the screen: its size is
// what the ptys get.
func (t *Terminal) paneRect() kit.Rect {
	if t.narrow() {
		return kit.Rect{X0: t.rect.X0 + 1, Y0: t.rect.Y0 + 1, Cols: t.rect.Cols - 2, Rows: t.bodyH() - 2}
	}
	lw := t.leftW()
	return kit.Rect{X0: t.rect.X0 + lw + 1, Y0: t.rect.Y0 + 1, Cols: t.rect.Cols - lw - 2, Rows: t.bodyH() - 2}
}

func (t *Terminal) PaneSize() (cols, rows int) {
	r := t.paneRect()
	return r.Cols, r.Rows
}

func (t *Terminal) View() string {
	on, _ := t.tree.Project()
	t.shared.Sync(&t.core.Selected, on.Name, t.tree.SelectProject)
	h, lw := t.bodyH(), t.leftW()
	left := func() string {
		focused := !t.capture.Held()
		rows := kit.WithSection(t.section(lw-2), h-2, func(lh int) []string { return t.list(lw-2, lh, focused) })
		return hits.Panel(1, kit.Box(kit.PanelTitle(1, "projects"), rows, lw, h, focused, false))
	}
	pane := func(w int) string {
		if r, ok := t.tree.Current(); ok && r.Shell == nil || t.pane.Session == nil {
			return hits.Panel(2, t.projectPanel(w, h))
		}
		box := kit.Box(kit.PanelTitle(2, t.pane.Title()), kit.ZoneBlock(hits.Pane, t.pane.View(w-2, h-2, t.capture.Held(), t.tick%2 == 0), w-2), w, h, t.capture.Held(), true)
		from, length := t.pane.Scrollbar(h - 2)
		return hits.Panel(2, kit.WithScrollbar(box, from, length))
	}
	switch {
	case t.narrow() && t.fullTerm:
		return pane(t.rect.Cols)
	case t.narrow():
		return left()
	}
	return kit.JoinHorizontal(left(), pane(t.rect.Cols-lw))
}

// list is the projects with their shells, drawn as the other tabs draw
// their trees.
func (t *Terminal) list(w, h int, focused bool) []string {
	rows := t.tree.Rows()
	if len(rows) == 0 {
		return []string{kit.StyleDim.Render(text.Fit(" none yet — Chat opens projects", w))}
	}
	cur, hasCur := t.tree.Current()
	selRow := -1
	blocks := make([][]string, len(rows))
	heights := make([]int, len(rows))
	project, shell := 0, 0
	for i, r := range rows {
		var lead, b []string
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
			entry := kit.ProjectHeading(r.Project.Name, r.Project.Path, kit.HeadLabel(t.core.Head(r.Project.Path)), w)
			if hasCur && t.tree.Moving && t.tree.Whole && cur.Project.Name == r.Project.Name {
				entry = kit.Picked(entry)
			}
			b = kit.DrawEntry(entry, w, false, focused)
			if i+1 < len(rows) && rows[i+1].Empty {
				empty := hasCur && cur.Empty && cur.Project.Name == r.Project.Name
				b = append(append(b, kit.ChildGap()), kit.DrawEntry(kit.EmptyEntry("no terminals yet"), w, empty, focused)...)
			}
			b = kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Heading, project), b, w)
		default:
			lead = []string{kit.ChildGap()}
			last := i+1 == len(rows) || rows[i+1].Heading()
			selected := hasCur && cur.Shell != nil && cur.Shell.Key == r.Shell.Key
			if selected {
				selRow = i
			}
			first, rest := "   ├─ ", "   │  "
			if last {
				first, rest = "   └─ ", "      "
			}
			// One mark, as in Chat: the spinner while the shell runs, ○ once it ended.
			spin := kit.Spinner[t.tick%len(kit.Spinner)]
			glyph, plain := kit.StyleBusy.Render(spin), spin
			if !t.act.Live.Running(r.Shell.Key) {
				glyph, plain = kit.StyleDim.Render("○"), "○"
			}
			entry := kit.TitleRows(first, rest, glyph+" ", plain+" ", r.Shell.Name, kit.StyleBold, w-2, nameLines)
			if selected && t.tree.Moving && !t.tree.Whole {
				entry = kit.Picked(entry)
			}
			b = kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Row, shell), kit.DrawEntry(entry, w, selected, focused), w)
			shell++
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

// projectPanel is the right side on a project's empty row: what n does there, and the
// project's shells by name. Nothing here takes the keys.
func (t *Terminal) projectPanel(w, h int) string {
	p, ok := t.tree.Project()
	if !ok {
		return kit.Box(kit.PanelTitle(2, "terminal"), nil, w, h, false, false)
	}
	var lines []string
	n := t.tree.Count(p.Name)
	if n == 0 {
		for _, l := range text.Wrap("No terminal in "+p.Name+": (n) opens a shell in "+text.ShortHome(p.Path)+".", w-4, "") {
			lines = append(lines, kit.StyleDim.Render(" "+l))
		}
	} else {
		for _, r := range t.tree.Rows() {
			if r.Shell != nil && r.Shell.Project == p.Name {
				lines = append(lines, " "+kit.StyleBold.Render(text.Fit(r.Shell.Name, w-4)))
			}
		}
	}
	return kit.Box(kit.PanelTitle(2, fmt.Sprintf("%s · terminals (%d)", p.Name, n)), append([]string{""}, lines...), w, h, false, false)
}

// section is the rows under the projects: the shell n starts and how many
// run, as Chat shows its AI tools there.
func (t *Terminal) section(w int) []string {
	program := filepath.Base(actions.Program())
	mark, detail := kit.StyleDim.Render("●"), "none running"
	if n := len(t.act.Live.Alive()); n > 0 {
		mark, detail = kit.StyleBusy.Render(kit.Spinner[t.tick%len(kit.Spinner)]), fmt.Sprintf("%d running", n)
	}
	return kit.SectionRows("shell", []kit.SectionItem{{Mark: mark, Name: program, Styled: kit.StyleAccent.Render(program), Detail: detail}}, w)
}
