package kit

import (
	"math"
	"strings"
)

// Charts drawn with block and braille cells. Each of the four kinds has a
// glyph as well as a colour (SeriesGlyphs, StyleSeries), so a stacked bar
// still reads where colour does not.

// SeriesGlyphs fill a kind's cells, lightest for the first kind.
var SeriesGlyphs = [4]string{"░", "▒", "▓", "█"}

// Seg is one kind's share of a stacked bar.
type Seg struct {
	Kind  int
	Value int64
}

// share splits n cells among values in proportion, the largest remainders
// rounded up, so the cells add up to n exactly.
func share(values []int64, n int) []int {
	var total int64
	for _, v := range values {
		total += max(0, v)
	}
	out := make([]int, len(values))
	if total == 0 || n <= 0 {
		return out
	}
	type rem struct {
		i int
		r float64
	}
	var rems []rem
	used := 0
	for i, v := range values {
		exact := float64(max(0, v)) * float64(n) / float64(total)
		out[i] = int(exact)
		used += out[i]
		rems = append(rems, rem{i, exact - float64(out[i])})
	}
	for used < n {
		best := -1
		for j, r := range rems {
			if best < 0 || r.r > rems[best].r {
				best = j
			}
		}
		out[rems[best].i]++
		rems[best].r = -1
		used++
	}
	return out
}

// StackedBar is segs as one row of w cells, scaled so scale fills them:
// a smaller total leaves the rest blank, so bars on rows compare.
func StackedBar(segs []Seg, scale int64, w int) string {
	if w <= 0 {
		return ""
	}
	var total int64
	values := make([]int64, len(segs))
	for i, s := range segs {
		values[i] = max(0, s.Value)
		total += values[i]
	}
	if scale < total {
		scale = total
	}
	filled := 0
	if scale > 0 {
		filled = int(math.Round(float64(total) * float64(w) / float64(scale)))
	}
	if total > 0 && filled == 0 {
		filled = 1
	}
	var b strings.Builder
	for i, n := range share(values, filled) {
		k := segs[i].Kind & 3
		b.WriteString(StyleSeries[k].Render(strings.Repeat(SeriesGlyphs[k], n)))
	}
	b.WriteString(strings.Repeat(" ", w-filled))
	return b.String()
}

// sparks are a sparkline's eight heights.
var sparks = []rune("▁▂▃▄▅▆▇█")

// Sparkline is the last w values as a row of eighth-heights, the largest
// full; zeros stay at the floor.
func Sparkline(values []int64, w int) string {
	if w <= 0 {
		return ""
	}
	if len(values) > w {
		values = values[len(values)-w:]
	}
	var top int64
	for _, v := range values {
		top = max(top, v)
	}
	var b strings.Builder
	for _, v := range values {
		i := 0
		if top > 0 && v > 0 {
			i = int(math.Ceil(float64(v)*8/float64(top))) - 1
		}
		b.WriteRune(sparks[min(max(i, 0), 7)])
	}
	return b.String() + strings.Repeat(" ", w-len(values))
}

// Columns is one stacked column per stack, h rows tall, the tallest stack
// (or scale, when larger) reaching the top; each cell takes the kind that
// covers most of it.
func Columns(stacks [][]Seg, scale int64, h int) []string {
	for _, st := range stacks {
		var t int64
		for _, s := range st {
			t += max(0, s.Value)
		}
		scale = max(scale, t)
	}
	rows := make([]strings.Builder, h)
	for _, st := range stacks {
		values := make([]int64, len(st))
		var total int64
		for i, s := range st {
			values[i] = max(0, s.Value)
			total += values[i]
		}
		cells := 0
		if scale > 0 {
			cells = int(math.Round(float64(total) * float64(h) / float64(scale)))
		}
		if total > 0 && cells == 0 {
			cells = 1
		}
		var col []string // bottom up
		for i, n := range share(values, cells) {
			k := st[i].Kind & 3
			for range n {
				col = append(col, StyleSeries[k].Render(SeriesGlyphs[k]))
			}
		}
		for r := range h {
			if from := h - 1 - r; from < len(col) {
				rows[r].WriteString(col[from])
			} else {
				rows[r].WriteString(" ")
			}
		}
	}
	out := make([]string, h)
	for i := range rows {
		out[i] = rows[i].String()
	}
	return out
}

// braille dot bits by column (0, 1) and row (0 top … 3 bottom).
var brailleDots = [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// Line is values as a braille line w cells wide and h tall, two points a
// cell across and four a cell down; the last 2w values are drawn, spread
// over the width when there are fewer, each joined to the next, the
// largest at the top.
func Line(values []int64, w, h int) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	if len(values) > 2*w {
		values = values[len(values)-2*w:]
	}
	var top int64
	for _, v := range values {
		top = max(top, v)
	}
	grid := make([][]rune, h)
	for r := range grid {
		grid[r] = []rune(strings.Repeat(string(rune(0x2800)), w))
	}
	dots, across := 4*h, 2*w
	level := func(v int64) int {
		if top == 0 {
			return dots - 1
		}
		return dots - 1 - int(math.Round(float64(max(0, v))*float64(dots-1)/float64(top)))
	}
	dot := func(x, row int) { grid[row/4][x/2] |= brailleDots[x%2][row%4] }
	at := func(i int) int {
		if len(values) < 2 {
			return 0
		}
		return i * (across - 1) / (len(values) - 1)
	}
	for i, v := range values {
		x, row := at(i), level(v)
		dot(x, row)
		if i+1 == len(values) {
			continue
		}
		// Join to the next point: a run across, then a step to its level.
		nx, nrow := at(i+1), level(values[i+1])
		for xx := x + 1; xx < nx; xx++ {
			dot(xx, row+(nrow-row)*(xx-x)/max(1, nx-x))
		}
		for r := min(row, nrow); r <= max(row, nrow); r++ {
			dot(nx, r)
		}
	}
	out := make([]string, h)
	for r, g := range grid {
		out[r] = StyleSeries[1].Render(string(g))
	}
	return out
}

// Legend names the kinds with their glyphs and colours.
func Legend(labels []string) string {
	var parts []string
	for i, l := range labels {
		k := i & 3
		parts = append(parts, StyleSeries[k].Render(SeriesGlyphs[k])+" "+l)
	}
	return strings.Join(parts, "   ")
}
