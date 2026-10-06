package ui

import (
	"bytes"
	"errors"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
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

// A resize clears the screen, the start screen's too: Terminal.app keeps a
// narrowed window's cells past the new width and showed the wider frame's
// right border beside the new one.
func TestResizeClears(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	_, cmd := d.app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd == nil || fmt.Sprintf("%T", cmd()) != fmt.Sprintf("%T", tea.ClearScreen()) {
		t.Errorf("the app's resize does not clear the screen")
	}
	d.quitApp()
	m := &setupModel{}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30}); cmd == nil || fmt.Sprintf("%T", cmd()) != fmt.Sprintf("%T", tea.ClearScreen()) {
		t.Errorf("the start screen's resize does not clear the screen")
	}
}

// A program runs with the terminal's line wrap off and gets it back however
// it ends: Terminal.app draws Bengali wider than counted, and a wrapped line
// pushed the whole screen up on every frame.
func TestNoWrap(t *testing.T) {
	var out bytes.Buffer
	failed := errors.New("the program ended badly")
	err := noWrap(&out, func() error {
		if out.String() != "\x1b[?7l" {
			t.Errorf("before the program: %q, want the wrap turned off", out.String())
		}
		return failed
	})
	if err != failed || out.String() != "\x1b[?7l\x1b[?7h" {
		t.Errorf("after the program: %q, %v; want the wrap back and its error", out.String(), err)
	}
}
