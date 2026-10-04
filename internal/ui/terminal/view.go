package terminal

import (
	"fmt"
	"path/filepath"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/terminal/actions"
	"lazychat/internal/ui/text"
)

// nameLines is how many rows a shell's name may wrap to.
const nameLines = 2

func (t *Terminal) View() string {
	on, _ := t.tree.Project()
	t.shared.Sync(&t.core.Selected, on.Name, t.tree.SelectProject)
	h, lw := t.BodyH(), t.LeftW()
	left := func() string {
		focused := !t.Capture.Held() && !t.PaneSel
		rows := kit.WithSection(t.section(lw-2), h-2, func(lh int) []string { return t.list(lw-2, lh, focused) })
		return hits.Panel(1, kit.Box(kit.PanelTitle(1, "projects"), rows, lw, h, focused, false))
	}
	pane := func(w int) string {
		if r, ok := t.tree.Current(); ok && r.Shell == nil || t.Pane.Session == nil {
			return hits.Panel(2, t.projectPanel(w, h))
		}
		box := kit.Box(kit.PanelTitle(2, t.Pane.Title()), kit.ZoneBlock(hits.Pane, t.Pane.View(w-2, h-2, t.Capture.Held(), t.Tick%2 == 0), w-2), w, h, t.Capture.Held() || t.PaneSel, true)
		from, length := t.Pane.Scrollbar(h - 2)
		return hits.Panel(2, kit.WithScrollbar(box, from, length))
	}
	switch {
	case t.Narrow() && t.FullTerm:
		return pane(t.Rect.Cols)
	case t.Narrow():
		return left()
	}
	return kit.JoinHorizontal(left(), pane(t.Rect.Cols-lw))
}

// list is the projects with their shells, drawn as the other tabs draw
// their trees.
func (t *Terminal) list(w, h int, focused bool) []string {
	rows := t.tree.Rows()
	if len(rows) == 0 {
		return []string{kit.StyleDim.Render(text.Fit(" none yet — Chat opens projects", w))}
	}
	cur, hasCur := t.tree.Current()
	blocks := make([]kit.TreeBlock, len(rows))
	project, shell := 0, 0
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
			entry := kit.ProjectHeading(r.Project.Name, r.Project.Path, kit.HeadLabel(t.core.Head(r.Project.Path)), w)
			if hasCur && t.tree.Moving && t.tree.Whole && cur.Project.Name == r.Project.Name {
				entry = kit.Picked(entry)
			}
			empty := i+1 < len(rows) && rows[i+1].Empty
			b.Rows = kit.HeadingRows(entry, empty, "no terminals yet", empty && hasCur && cur.Empty && cur.Project.Name == r.Project.Name, w, focused, fmt.Sprintf("%s-%d", hits.Heading, project))
		default:
			b.Lead = []string{kit.ChildGap()}
			last := i+1 == len(rows) || rows[i+1].Heading()
			b.Selected = hasCur && cur.Shell != nil && cur.Shell.Key == r.Shell.Key
			first, rest := "   ├─ ", "   │  "
			if last {
				first, rest = "   └─ ", "      "
			}
			// One mark, as in Chat: the spinner while the shell runs, ○ once it ended.
			spin := kit.Spinner[t.Tick%len(kit.Spinner)]
			glyph, plain := kit.StyleBusy.Render(spin), spin
			if !t.act.Live.Running(r.Shell.Key) {
				glyph, plain = kit.StyleDim.Render("○"), "○"
			}
			entry := kit.TitleRows(first, rest, glyph+" ", plain+" ", r.Shell.Name, kit.StyleBold, w-2, nameLines)
			if b.Selected && t.tree.Moving && !t.tree.Whole {
				entry = kit.Picked(entry)
			}
			b.Rows = kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Row, shell), kit.DrawEntry(entry, w, b.Selected, focused), w)
			shell++
		}
	}
	return kit.DrawTree(blocks, &t.scroll, h)
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
		mark, detail = kit.StyleBusy.Render(kit.Spinner[t.Tick%len(kit.Spinner)]), fmt.Sprintf("%d running", n)
	}
	return kit.SectionRows("shell", []kit.SectionItem{{Mark: mark, Name: program, Styled: kit.StyleAccent.Render(program), Detail: detail}}, w)
}
