// Package vm is the state that more than one tab's model shares, in plain
// Go: no Bubble Tea, no styling, so every model can use it and its tests
// run without a screen.
package vm

// List is the cursor and scroll offset of one list; the trees and the
// picker share the arithmetic so they scroll and clamp the same way.
type List struct {
	Sel, Scroll int
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	return max(lo, min(hi, v))
}

func (l *List) ClampTo(n int) { l.Sel = clamp(l.Sel, 0, max(0, n-1)) }

func (l *List) Move(d, n int) {
	l.Sel = clamp(l.Sel+d, 0, max(0, n-1))
}

// Step moves the cursor d rows that take it, passing over the ones that do
// not (a project's heading, a section's label); it stops at the last such
// row either way.
func (l *List) Step(d, n int, takes func(i int) bool) {
	l.Settle(n, takes)
	dir := 1
	if d < 0 {
		dir, d = -1, -d
	}
	for ; d > 0; d-- {
		i := l.Sel + dir
		for i >= 0 && i < n && !takes(i) {
			i += dir
		}
		if i < 0 || i >= n {
			return
		}
		l.Sel = i
	}
}

// Settle puts a cursor left on a row that takes none on the nearest one
// that does, below it first, as a list reads on after a row goes.
func (l *List) Settle(n int, takes func(i int) bool) {
	l.ClampTo(n)
	if n == 0 || takes(l.Sel) {
		return
	}
	for d := 1; d < n; d++ {
		if i := l.Sel + d; i < n && takes(i) {
			l.Sel = i
			return
		}
		if i := l.Sel - d; i >= 0 && takes(i) {
			l.Sel = i
			return
		}
	}
}

// ScrollBlocks moves the scroll offset so the cursor's block is on screen when
// blocks have different heights; it returns the offset to draw from.
func (l *List) ScrollBlocks(heights []int, avail int) int {
	l.ClampTo(len(heights))
	if l.Sel < l.Scroll {
		l.Scroll = l.Sel
	}
	for l.Scroll < l.Sel {
		used := 0
		for i := l.Scroll; i <= l.Sel; i++ {
			used += heights[i]
		}
		if used <= avail {
			break
		}
		l.Scroll++
	}
	return l.Scroll
}

// Window returns the first and one-past-last item index that fit in avail
// rows of rowsPerItem each, keeping the cursor visible.
func (l *List) Window(n, avail, rowsPerItem int) (start, end int) {
	perPage := max(1, avail/max(1, rowsPerItem))
	if l.Sel < l.Scroll {
		l.Scroll = l.Sel
	}
	if l.Sel >= l.Scroll+perPage {
		l.Scroll = l.Sel - perPage + 1
	}
	l.Scroll = clamp(l.Scroll, 0, max(0, n-1))
	return l.Scroll, min(n, l.Scroll+perPage)
}

// Scroller is a tree's scroll offset, kept between frames. It follows the
// cursor, except after the wheel moved it: then it stays where the wheel
// left it, so the list can be read without changing what the cursor shows,
// until the cursor moves or Follow is called.
type Scroller struct {
	Top     int
	free    bool
	sel     int
	last    int
	heights []int // the last frame's, so the wheel passes blocks that draw nothing
}

// Wheel moves the offset by d blocks, leaving the cursor where it is. It
// stops at the ends of what the last frame drew, so turns past the end are
// not owed back before the list moves the other way. A block of no rows (a
// row drawn inside its heading's) is not a turn of its own.
func (s *Scroller) Wheel(d int) {
	dir := 1
	if d < 0 {
		dir, d = -1, -d
	}
	for ; d > 0; d-- {
		s.Top = clamp(s.Top+dir, 0, s.last)
		for s.Top > 0 && s.Top < s.last && s.Top < len(s.heights) && s.heights[s.Top] == 0 {
			s.Top += dir
		}
	}
	s.free = true
}

// Follow brings the cursor's row back into view on the next frame.
func (s *Scroller) Follow() { s.free = false }

// Place returns the first block to draw. Following the cursor, it keeps the
// cursor's block in view, and the one above too when headingAbove says that
// is the heading the cursor's block hangs off.
func (s *Scroller) Place(heights []int, sel, avail int, headingAbove bool) int {
	s.last = lastTop(heights, avail)
	s.heights = heights
	if s.free && sel == s.sel {
		s.Top = clamp(s.Top, 0, s.last)
		return s.Top
	}
	s.free, s.sel = false, sel
	v := List{Sel: max(sel, 0), Scroll: s.Top}
	from := v.ScrollBlocks(heights, avail)
	if headingAbove && from == sel && sel > 0 && heights[sel-1]+heights[sel] <= avail {
		from--
	}
	s.Top = from
	return from
}

// lastTop is the largest offset that still fills avail rows, so the wheel
// stops when the last block is at the bottom.
func lastTop(heights []int, avail int) int {
	used := 0
	for i := len(heights) - 1; i >= 0; i-- {
		used += heights[i]
		if used > avail {
			return i + 1
		}
	}
	return 0
}
