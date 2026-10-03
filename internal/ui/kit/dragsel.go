package kit

import (
	"strings"

	"lazychat/internal/ui/text"
)

// Drag is a mouse selection over the pane's history, from the cell a press
// landed on to the one under the pointer, both inclusive; rows are history
// indices, so it holds still while output arrives. Programs on the
// alternate screen keep the mouse for themselves and never get one.
type Drag struct {
	Active   bool
	dragging bool
	fromRow  int
	fromCol  int
	toRow    int
	toCol    int
	copied   int // characters the last release copied, for the title
}

// span is the selection in reading order.
func (d *Drag) span() (r0, c0, r1, c1 int) {
	r0, c0, r1, c1 = d.fromRow, d.fromCol, d.toRow, d.toCol
	if r1 < r0 || r1 == r0 && c1 < c0 {
		r0, c0, r1, c1 = r1, c1, r0, c0
	}
	return r0, c0, r1, c1
}

// top is the history index of the pane's first row.
func (p *TermPane) top(r Rect) int {
	if p.Live {
		return max(0, p.Session.Total()-r.Rows)
	}
	return p.Top
}

// Press starts a selection at the pane cell (px, py); the view holds still
// until the selection ends, so the text under the pointer stays put.
func (p *TermPane) Press(r Rect, px, py int) {
	if p.Session == nil || p.Session.AltScreen() {
		return
	}
	p.Top = p.top(r)
	p.Live, p.Sel.Active = false, false
	row := p.Top + py
	p.Drag = Drag{Active: true, dragging: true, fromRow: row, fromCol: px, toRow: row, toCol: px}
}

// DragTo moves the selection's end; above or below the pane it scrolls.
func (p *TermPane) DragTo(r Rect, px, py int) {
	if !p.Drag.dragging {
		return
	}
	switch {
	case py < 0:
		p.Top = max(0, p.Top-1)
		py = 0
	case py >= r.Rows:
		p.Top = min(p.Top+1, max(0, p.Session.Total()-r.Rows))
		py = r.Rows - 1
	}
	p.Drag.toRow, p.Drag.toCol = p.Top+py, Clamp(px, 0, r.Cols-1)
}

// Release ends the selection and copies it; a click with no drag selects
// nothing and the pane follows the program again.
func (p *TermPane) Release() {
	if !p.Drag.dragging {
		return
	}
	p.Drag.dragging = false
	if p.Drag.fromRow == p.Drag.toRow && p.Drag.fromCol == p.Drag.toCol {
		p.Drag.Active = false
		p.Live = true
		return
	}
	sel := p.DragText()
	if err := CopyToClipboard(sel); err == nil {
		p.Drag.copied = len([]rune(sel))
	}
}

// DragText is the plain text the selection covers: the first row from its
// cell, the last up to its cell, whole rows between, trailing blanks off.
func (p *TermPane) DragText() string {
	r0, c0, r1, c1 := p.Drag.span()
	_, plain := p.Session.Rows(r0, r1+1)
	var out []string
	for i, line := range plain {
		rs := []rune(line)
		from, to := 0, len(rs)
		if i == 0 {
			from = min(c0, len(rs))
		}
		if i == len(plain)-1 {
			to = min(c1+1, len(rs))
		}
		if from > to {
			from = to
		}
		out = append(out, strings.TrimRight(string(rs[from:to]), " "))
	}
	return strings.Join(out, "\n")
}

// dragRows draws the rows from the history with the selection painted.
func (p *TermPane) dragRows(w, h int) []string {
	styled, plain := p.Session.Rows(p.Top, p.Top+h)
	r0, c0, r1, c1 := p.Drag.span()
	for i := range styled {
		abs := p.Top + i
		if abs < r0 || abs > r1 {
			continue
		}
		rs := []rune(text.Pad(text.FitExact(plain[i], w), w))
		from, to := 0, len(rs)-1
		if abs == r0 {
			from = min(c0, len(rs)-1)
		}
		if abs == r1 {
			to = min(c1, len(rs)-1)
		}
		if from > to {
			continue
		}
		styled[i] = string(rs[:from]) + StyleSel.Render(string(rs[from:to+1])) + string(rs[to+1:])
	}
	return styled
}
