package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A stacked bar fills its cells in proportion, every kind with its own
// glyph, and a smaller total leaves the rest blank; widths add up exactly.
func TestStackedBar(t *testing.T) {
	bar := ansi.Strip(StackedBar([]Seg{{0, 50}, {3, 50}}, 100, 10))
	if bar != "░░░░░█████" {
		t.Errorf("half and half: %q", bar)
	}
	if bar := ansi.Strip(StackedBar([]Seg{{1, 10}}, 100, 10)); bar != "▒         " {
		t.Errorf("a tenth: %q", bar)
	}
	if bar := ansi.Strip(StackedBar([]Seg{{2, 1}}, 1_000_000, 10)); bar != "▓         " {
		t.Errorf("a sliver still shows: %q", bar)
	}
	for w := range 7 {
		if got := len([]rune(ansi.Strip(StackedBar([]Seg{{0, 3}, {1, 3}, {2, 3}}, 0, w)))); got != w {
			t.Errorf("width %d drew %d cells", w, got)
		}
	}
	if bar := ansi.Strip(StackedBar(nil, 0, 4)); bar != "    " {
		t.Errorf("nothing: %q", bar)
	}
}

// A sparkline keeps the last w values, the largest full height.
func TestSparkline(t *testing.T) {
	if got := Sparkline([]int64{0, 1, 4, 8}, 4); got != "▁▁▄█" {
		t.Errorf("sparkline %q", got)
	}
	if got := Sparkline([]int64{9, 9, 1, 2}, 2); got != "▄█" {
		t.Errorf("the last two %q", got)
	}
	if got := Sparkline(nil, 3); got != "   " {
		t.Errorf("empty %q", got)
	}
}

// Columns stack each kind bottom up, the tallest reaching the top.
func TestColumns(t *testing.T) {
	rows := Columns([][]Seg{{{0, 2}, {3, 2}}, {{0, 1}}}, 0, 4)
	var plain []string
	for _, r := range rows {
		plain = append(plain, ansi.Strip(r))
	}
	if strings.Join(plain, "|") != "█ |█ |░ |░░" {
		t.Errorf("columns %q", plain)
	}
}

// A braille line puts the largest value on the top row and the smallest
// on the bottom.
func TestLine(t *testing.T) {
	rows := Line([]int64{0, 10}, 1, 2)
	top, bottom := []rune(ansi.Strip(rows[0]))[0], []rune(ansi.Strip(rows[1]))[0]
	if top&0x08 == 0 || bottom&0x40 == 0 {
		t.Errorf("line %q %q", ansi.Strip(rows[0]), ansi.Strip(rows[1]))
	}
}
