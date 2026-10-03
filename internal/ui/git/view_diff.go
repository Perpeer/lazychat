package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// line is one screen row of a diff: a row of a file, or, when a patch
// holds several files, the file's own heading.
type line struct {
	file string // set on a file heading
	row  git.Row
	w    int // the width of the numbers in this line's file
}

// flatten lays a patch's files out as screen rows; a single file needs no
// heading, the panel's title names it.
func flatten(files []git.File) []line {
	var out []line
	for _, f := range files {
		w := 3
		for _, r := range f.Rows {
			w = max(w, len(strconv.Itoa(max(r.Old, r.New))))
		}
		if len(files) > 1 {
			name := f.Path
			if f.Orig != "" {
				name = f.Orig + " → " + f.Path
			}
			out = append(out, line{file: name})
		}
		for _, r := range f.Rows {
			out = append(out, line{row: r, w: w})
		}
	}
	return out
}

// tabWidth is how many columns a tab in a diff takes.
const tabWidth = 4

// drawDiff draws rows from..from+h of lines, w wide: only what is on
// screen, however long the diff is.
func drawDiff(lines []line, from, w, h int) []string {
	var out []string
	for i := from; i < len(lines) && len(out) < h; i++ {
		out = append(out, drawLine(lines[i], w))
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
	return gutter + paint(body, []rune(r.Text), r.Changed, base, word, w-text.Width(gutter))
}

// paint draws a row's text after its mark in the row's colour, the changed
// spans in the stronger one, tabs as spaces, cut and filled to w columns so
// the colour runs to the panel's edge.
func paint(mark string, rs []rune, spans []git.Span, base, word lipgloss.Style, w int) string {
	var b strings.Builder
	used := 0
	cur, run := -1, strings.Builder{}
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if cur == 1 {
			b.WriteString(word.Render(run.String()))
		} else {
			b.WriteString(base.Render(run.String()))
		}
		run.Reset()
	}
	put := func(s string, strong int) bool {
		sw := text.Width(s)
		if used+sw > w {
			return false
		}
		if strong != cur {
			flush()
			cur = strong
		}
		run.WriteString(s)
		used += sw
		return true
	}
	if !put(mark, 0) {
		flush()
		return b.String()
	}
	for i, c := range rs {
		strong := 0
		for _, s := range spans {
			if i >= s.From && i < s.To {
				strong = 1
			}
		}
		s := string(c)
		if c == '\t' {
			s = strings.Repeat(" ", tabWidth)
		}
		if !put(s, strong) {
			break
		}
	}
	if used < w {
		put(strings.Repeat(" ", w-used), 0)
	}
	flush()
	return b.String()
}
