package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A frame is never wider or taller than the terminal: a line the terminal
// would wrap, or a row too many, would push the whole screen up. Lines are
// cut, styles and wide characters kept whole where they fit.
func TestFrameFits(t *testing.T) {
	wide := "\x1b[31m" + strings.Repeat("界", 30) + "\x1b[0m" // 60 columns of wide characters
	frame := strings.Join([]string{"short", wide, "plain " + strings.Repeat("x", 80), "4", "5", "6"}, "\n")
	got := strings.Split(fitFrame(frame, 40, 4), "\n")
	if len(got) != 4 {
		t.Fatalf("%d rows, want 4", len(got))
	}
	for i, l := range got {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("row %d is %d wide: %q", i, w, ansi.Strip(l))
		}
	}
	if got[0] != "short" || !strings.HasPrefix(ansi.Strip(got[1]), "界界") || !strings.HasSuffix(got[1], "\x1b[0m") && strings.Count(ansi.Strip(got[1]), "界") != 20 {
		t.Errorf("rows %q", got)
	}
	if fitFrame("a\nb", 0, 0) != "a\nb" {
		t.Error("no size yet: the frame is left as it is")
	}
}

// The whole app's frame fits the terminal at every size, a narrow one
// included.
func TestAppFrameFits(t *testing.T) {
	e, _ := seeded(t)
	for _, size := range [][2]int{{120, 32}, {80, 24}, {200, 60}} {
		d := start(t, e, size[0], size[1])
		d.expect("[1] projects")
		rows := strings.Split(d.app.View(), "\n")
		if len(rows) > size[1] {
			t.Errorf("%dx%d: %d rows", size[0], size[1], len(rows))
		}
		for i, r := range rows {
			if w := ansi.StringWidth(r); w > size[0] {
				t.Errorf("%dx%d: row %d is %d wide", size[0], size[1], i, w)
			}
		}
		d.quitApp()
	}
}
