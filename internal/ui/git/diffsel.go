package git

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	coregit "lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
)

// The diff's rows can be walked and copied, so a part of a change can be
// handed to a prompt: a cursor while the diff has the keys, v marks one
// end, y copies the span, and a mouse drag selects and copies on release.

// diffSpan is the selected rows, the cursor's alone with no mark.
func (g *Git) diffSpan() (lo, hi int) {
	if !g.diff.marked {
		return g.diff.cur, g.diff.cur
	}
	return min(g.diff.anchor, g.diff.cur), max(g.diff.anchor, g.diff.cur)
}

// diffLit says a row is drawn selected: the span, while the diff has the
// keys or a drag runs.
func (g *Git) diffLit(i int) bool {
	if g.focus != panelDiff && !g.diff.dragging {
		return false
	}
	lo, hi := g.diffSpan()
	return i >= lo && i <= hi
}

// moveDiff steps the cursor, the view following it.
func (g *Git) moveDiff(d int) {
	n := len(g.diff.lines)
	if n == 0 {
		return
	}
	g.diff.cur = kit.Clamp(g.diff.cur+d, 0, n-1)
	rows := g.diffRows()
	switch {
	case g.diff.cur < g.diff.top:
		g.diff.top = g.diff.cur
	case g.diff.cur >= g.diff.top+rows:
		g.diff.top = g.diff.cur - rows + 1
	}
}

// markDiff starts a selection at the cursor, or drops the one there is.
func (g *Git) markDiff() {
	g.diff.anchor, g.diff.marked = g.diff.cur, !g.diff.marked
}

// copyDiff puts the selected rows on the clipboard and says so in the
// status area; the mark is dropped after.
func (g *Git) copyDiff() {
	lo, hi := g.diffSpan()
	out, n := diffText(g.diff.lines, lo, hi)
	if n == 0 {
		g.screen.Note("nothing to copy there: no line of a file is selected")
		return
	}
	if err := kit.CopyToClipboard(out); err != nil {
		g.screen.Note("copy: %v", err)
		return
	}
	g.diff.marked = false
	g.screen.Note("copied %d line(s)", n)
}

// diffText is rows lo..hi as a prompt takes them: each file's part headed
// by its path and the span of its line numbers, every line marked + added,
// - removed or a space; hunk and file headings only part them. n is the
// lines it holds.
func diffText(lines []line, lo, hi int) (string, int) {
	var out []string
	var group []string
	path, first, last, n := "", 0, 0, 0
	flush := func() {
		if len(group) == 0 {
			return
		}
		head := path
		if first > 0 {
			head = fmt.Sprintf("%s:%d", path, first)
			if last > first {
				head += fmt.Sprintf("-%d", last)
			}
		}
		out = append(out, head)
		out = append(out, group...)
		group, first, last = nil, 0, 0
	}
	for i := max(0, lo); i <= hi && i < len(lines); i++ {
		l := lines[i]
		if l.file != "" || l.path != path {
			flush()
			path = l.path
		}
		mark := "  "
		switch l.row.Kind {
		case coregit.Added:
			mark = "+ "
		case coregit.Removed:
			mark = "- "
		case coregit.Context:
		default:
			continue
		}
		if l.file != "" {
			continue
		}
		if num := l.row.New; num > 0 {
			if first == 0 {
				first = num
			}
			last = num
		}
		group = append(group, mark+l.row.Text)
		n++
	}
	flush()
	return strings.Join(out, "\n"), n
}

// diffRowAt is the diff row under the mouse, kept within the diff.
func (g *Git) diffRowAt(msg tea.MouseMsg) int {
	top := g.rect.Y0 + 1 // the diff box's first inner row
	return kit.Clamp(g.diff.top+msg.Y-top, 0, max(0, len(g.diff.lines)-1))
}

// pressDiff starts a drag over the diff at the row pressed.
func (g *Git) pressDiff(msg tea.MouseMsg) {
	if len(g.diff.lines) == 0 {
		return
	}
	row := g.diffRowAt(msg)
	g.diff.cur, g.diff.anchor, g.diff.marked, g.diff.dragging = row, row, true, true
}

// dragDiff follows a drag over the diff; its release copies what it
// covers, and a click with no drag leaves the cursor on the row.
func (g *Git) dragDiff(msg tea.MouseMsg) tea.Cmd {
	switch msg.Action {
	case tea.MouseActionMotion:
		g.moveDiff(g.diffRowAt(msg) - g.diff.cur)
	case tea.MouseActionRelease:
		g.diff.dragging = false
		if g.diff.anchor == g.diff.cur {
			g.diff.marked = false
			return nil
		}
		g.copyDiff()
	}
	return nil
}

// stagedLinesMsg is the answer of staging lines: what was done to how many.
type stagedLinesMsg struct {
	did string
	n   int
	err error
}

// stageLines is space in the diff: the selected rows, or the cursor's,
// into the index — out of it in the staged diff — as exactly those lines
// (core/git.ApplyLines). A selection over two files, a commit's diff and
// a heading row are refused; the diff reloads after, its selection
// dropped, so stale rows are never applied twice.
func (g *Git) stageLines() tea.Cmd {
	lo, hi := g.diffSpan()
	n := len(g.diff.lines)
	if n == 0 || lo < 0 || hi >= n || strings.HasPrefix(g.diff.key, "commit ") {
		return nil
	}
	first := g.diff.lines[lo]
	for i := lo; i <= hi; i++ {
		if l := g.diff.lines[i]; l.file != "" || l.fi != first.fi {
			g.screen.Note("select lines of one file to stage them")
			return nil
		}
	}
	if first.fi >= len(g.diff.files) {
		return nil
	}
	p := g.cursorStatus()
	if p == nil {
		return nil
	}
	file := g.diff.files[first.fi]
	untracked := false
	for _, e := range p.st.Entries {
		if e.Path == file.Path {
			untracked = e.Untracked
		}
	}
	lines := coregit.Lines{File: file, Lo: first.ri, Hi: g.diff.lines[hi].ri, Reverse: g.diff.staged}
	root, did := p.st.Root, "staged"
	if lines.Reverse {
		did = "unstaged"
	}
	count := 0
	for i := lo; i <= hi; i++ {
		if k := g.diff.lines[i].row.Kind; k == coregit.Added || k == coregit.Removed {
			count++
		}
	}
	return g.own(func() tea.Msg {
		return stagedLinesMsg{did: did, n: count, err: coregit.ApplyLines(root, lines, untracked)}
	})
}
