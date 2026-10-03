package kit

import "strings"

// CopyMode is a selection of whole rows over the pane's history: a cursor
// row and, once marked, an anchor. Rows are history indices, so the
// selection holds still while output arrives.
type CopyMode struct {
	Active bool
	Cursor int
	Anchor int // -1 until marked
}

func (c *CopyMode) Mark() {
	if c.Anchor < 0 {
		c.Anchor = c.Cursor
	} else {
		c.Anchor = -1
	}
}

// Span is the first and last selected row.
func (c *CopyMode) Span() (lo, hi int) {
	if c.Anchor < 0 {
		return c.Cursor, c.Cursor
	}
	return min(c.Anchor, c.Cursor), max(c.Anchor, c.Cursor)
}

func (p *TermPane) Copying() bool { return p.Sel.Active }

func (p *TermPane) StopCopy() { p.Sel.Active = false }

// StartCopy puts a cursor on the bottom row of what is shown.
func (p *TermPane) StartCopy(h int) {
	if p.Session == nil || p.Session.AltScreen() {
		return
	}
	if p.Live {
		p.Top = max(0, p.Session.Total()-h)
	}
	p.Live, p.Sel.Active = false, true
	// Start on the last row with text: the bottom of claude's screen is often blank.
	p.Sel.Cursor, p.Sel.Anchor = p.Top+h-1, -1
	_, plain := p.Session.Rows(p.Top, p.Top+h)
	for i := len(plain) - 1; i > 0 && plain[i] == ""; i-- {
		p.Sel.Cursor = p.Top + i - 1
	}
}

// MoveCursor steps the copy cursor, scrolling the window to keep it in view.
func (p *TermPane) MoveCursor(n, h int) {
	p.Sel.Cursor = Clamp(p.Sel.Cursor+n, 0, p.Session.Total()-1)
	if p.Sel.Cursor < p.Top {
		p.Top = p.Sel.Cursor
	}
	if p.Sel.Cursor >= p.Top+h {
		p.Top = p.Sel.Cursor - h + 1
	}
}

// Selected is the plain text of the selected rows.
func (p *TermPane) Selected() string {
	lo, hi := p.Sel.Span()
	_, plain := p.Session.Rows(lo, hi+1)
	return strings.Join(plain, "\n")
}
