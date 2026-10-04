package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/git"
	"lazychat/internal/core/syntax"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// line is one screen row of a diff: a row of a file, or, when a patch
// holds several files, the file's own heading.
type line struct {
	file string // set on a file heading
	path string // the file every row belongs to, for a copy's header
	row  git.Row
	w    int           // the width of the numbers in this line's file
	syn  []syntax.Span // its code's colours; nil plain
	fi   int           // which file, and which of its rows, for staging by line
	ri   int
}

// flatten lays a patch's files out as screen rows; a single file needs no
// heading, the panel's title names it. roles, when given, are each file's
// code colours by row (syntaxOf).
func flatten(files []git.File, roles ...[][]syntax.Span) []line {
	var out []line
	for fi, f := range files {
		w := 3
		for _, r := range f.Rows {
			w = max(w, len(strconv.Itoa(max(r.Old, r.New))))
		}
		if len(files) > 1 {
			name := f.Path
			if f.Orig != "" {
				name = f.Orig + " → " + f.Path
			}
			out = append(out, line{file: name, path: f.Path})
		}
		for ri, r := range f.Rows {
			l := line{path: f.Path, row: r, w: w, fi: fi, ri: ri}
			if fi < len(roles) && ri < len(roles[fi]) {
				l.syn = roles[fi][ri]
			}
			out = append(out, l)
		}
	}
	return out
}

// syntaxOf is every file's code colours, read where the patch is read:
// tokenizing a large diff takes longer than a frame.
func syntaxOf(files []git.File) [][][]syntax.Span {
	out := make([][][]syntax.Span, len(files))
	for i, f := range files {
		out[i] = syntax.File(f)
	}
	return out
}

// tabWidth is how many columns a tab in a diff takes.
const tabWidth = 4

// drawDiff draws rows from..from+h of lines, w wide: only what is on
// screen, however long the diff is; the rows lit says are drawn selected,
// and code is coloured only while colour says so, so the switch acts at once.
func drawDiff(lines []line, from, w, h int, lit func(i int) bool, colour bool) []string {
	var out []string
	for i := from; i < len(lines) && len(out) < h; i++ {
		l := lines[i]
		if !colour {
			l.syn = nil
		}
		if lit != nil && lit(i) {
			out = append(out, kit.StyleSel.Render(text.Pad(ansi.Strip(drawLine(l, w)), w)))
			continue
		}
		out = append(out, drawLine(l, w))
	}
	return out
}

func drawLine(l line, w int) string {
	if l.file != "" {
		return kit.StyleBold.Render(text.Fit(" "+l.file, w))
	}
	r := l.row
	num := func(n int) string {
		if n == 0 {
			return strings.Repeat(" ", l.w)
		}
		return fmt.Sprintf("%*d", l.w, n)
	}
	switch r.Kind {
	case git.Hunk:
		return kit.StyleDim.Render(text.Fit(strings.Repeat(" ", 2*l.w+1)+" ⋯ "+r.Text, w))
	case git.Meta:
		return kit.StyleDim.Render(text.Fit(strings.Repeat(" ", 2*l.w+1)+"   "+r.Text, w))
	}
	mark, base, word := " ", lipgloss.NewStyle(), lipgloss.NewStyle()
	switch r.Kind {
	case git.Removed:
		mark, base, word = "-", kit.StyleRemoved, kit.StyleRemovedWord
	case git.Added:
		mark, base, word = "+", kit.StyleAdded, kit.StyleAddedWord
	}
	gutter := kit.StyleDim.Render(num(r.Old)+" "+num(r.New)) + " "
	body := mark + " "
	return gutter + paint(body, []rune(r.Text), r.Changed, l.syn, base, word, w-text.Width(gutter))
}

// paint draws a row's text after its mark in the row's colour, the changed
// spans in the stronger one, its code in the theme's syntax colours over
// them, tabs as spaces, cut and filled to w columns so the colour runs to
// the panel's edge.
func paint(mark string, rs []rune, spans []git.Span, syn []syntax.Span, base, word lipgloss.Style, w int) string {
	var b strings.Builder
	used := 0
	type look struct {
		strong bool
		role   syntax.Role
	}
	cur, run := look{role: 255}, strings.Builder{}
	flush := func() {
		if run.Len() == 0 {
			return
		}
		st := base
		if cur.strong {
			st = word
		}
		if cur.role != syntax.Plain {
			st = st.Foreground(kit.SyntaxColors[cur.role])
		}
		b.WriteString(st.Render(run.String()))
		run.Reset()
	}
	put := func(s string, lk look) bool {
		sw := text.Width(s)
		if used+sw > w {
			return false
		}
		if lk != cur {
			flush()
			cur = lk
		}
		run.WriteString(s)
		used += sw
		return true
	}
	if !put(mark, look{}) {
		flush()
		return b.String()
	}
	for i, c := range rs {
		lk := look{}
		for _, s := range spans {
			if i >= s.From && i < s.To {
				lk.strong = true
			}
		}
		for _, s := range syn {
			if i >= s.From && i < s.To {
				lk.role = s.Role
				break
			}
		}
		s := string(c)
		if c == '\t' {
			s = strings.Repeat(" ", tabWidth)
		}
		if !put(s, lk) {
			break
		}
	}
	if used < w {
		put(strings.Repeat(" ", w-used), look{})
	}
	flush()
	return b.String()
}
