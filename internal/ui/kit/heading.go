package kit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/text"
)

// dirLines is how many rows a heading's directory may wrap to; a longer one
// keeps its end, the part that tells projects apart. nameLines is the same
// for its name, which keeps its start.
const (
	dirLines  = 3
	nameLines = 2
)

// ChildGap is the row above each entry hanging off a project — a session,
// a shell, a branch — so entries tell apart at a glance; the tree's line runs
// through it, so they still read as one branch of their project.
func ChildGap() string { return StyleDim.Render("   │") }

// EmptyEntry is the row under a project that has nothing in it yet, where
// the cursor stands since headings take none; say names what n makes.
func EmptyEntry(say string) []TreeLine {
	return []TreeLine{{Prefix: "   └─ ", Styled: StyleDim.Render(say), Plain: say}}
}

// TitleRows lays a title out as tree rows: the first after first and the
// lead (a glyph), the others after rest, under the title's own start. Every
// row is drawn in style, so a wrapped name still reads as one name.
func TitleRows(first, rest string, lead, plainLead, title string, style lipgloss.Style, w, n int) []TreeLine {
	indent := strings.Repeat(" ", text.Width(plainLead))
	rows := text.WrapTitle(title, w-text.Width(first)-text.Width(plainLead), n)
	out := make([]TreeLine, len(rows))
	for i, r := range rows {
		if i == 0 {
			out[i] = TreeLine{Prefix: first, Styled: lead + style.Render(r), Plain: plainLead + r}
		} else {
			out[i] = TreeLine{Prefix: rest, Styled: indent + style.Render(r), Plain: indent + r}
		}
	}
	return out
}

// ProjectHeading is a project's heading in a column of projects: its name,
// the directory dim under it from the same column, and under that the
// branch it works on when one is given.
// Every tab draws projects this way, so a project looks the same wherever
// it is.
func ProjectHeading(name, path, branch string, w int) []TreeLine {
	names := text.WrapTitle(name, w-2, nameLines)
	out := []TreeLine{{Styled: StyleBold.Render(" " + names[0]), Plain: " " + names[0]}}
	for _, n := range names[1:] {
		out = append(out, TreeLine{Styled: StyleBold.Render(" " + n), Plain: " " + n})
	}
	// The folder starts where the name does.
	for _, d := range dirRows(text.ShortHome(path), w-2) {
		out = append(out, TreeLine{Styled: StyleDim.Render(" " + d), Plain: " " + d})
	}
	if branch != "" {
		b := text.Fit(branch, w-2)
		out = append(out, TreeLine{Styled: " " + StyleAccent.Render(b), Plain: " " + b})
	}
	return out
}

// HeadLabel is a project heading's branch: ⎇ and the branch, ⑂ when the
// folder is a worktree; "" outside a repository.
func HeadLabel(h git.Head, ok bool) string {
	switch {
	case !ok:
		return ""
	case h.Linked:
		return "⑂ " + h.Branch
	}
	return "⎇ " + h.Branch
}

// ListWidth is the projects column's width in a tab cols wide, the same in
// every tab so the column does not jump when the tab changes.
func ListWidth(cols int) int { return Clamp(cols*28/100, 32, 40) }

// dirRows breaks a path into rows of w columns, character by character —
// paths have no spaces to break at — keeping the last rows when it is longer.
func dirRows(path string, w int) []string {
	var rows []string
	cur := ""
	for _, r := range path {
		if text.Width(cur+string(r)) > w {
			rows = append(rows, cur)
			cur = ""
		}
		cur += string(r)
	}
	rows = append(rows, cur)
	if len(rows) > dirLines {
		rows = rows[len(rows)-dirLines:]
		rows[0] = text.FitLeft("…"+rows[0], w)
	}
	return rows
}
