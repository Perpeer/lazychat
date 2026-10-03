package kit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var editorKeys = func() map[string]tea.KeyType {
	m := map[string]tea.KeyType{}
	for k := tea.KeyType(-200); k < 128; k++ {
		if name := (tea.Key{Type: k}).String(); name != "" && k != tea.KeyRunes {
			if _, ok := m[name]; !ok {
				m[name] = k
			}
		}
	}
	return m
}()

// press sends keys by name; anything else is typed as runes.
func press(t *testing.T, e *Editor, keys ...string) {
	t.Helper()
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if kt, ok := editorKeys[k]; ok && len([]rune(k)) > 1 {
			msg = tea.KeyMsg{Type: kt}
		}
		if !e.Key(msg) {
			t.Fatalf("key %q not taken", k)
		}
	}
}

// Editing and moving: each case starts from the same text with the cursor
// at the start, presses keys, and says the text, the cursor and the
// selection it ends with.
func TestEditorKeys(t *testing.T) {
	const text = "first line\nsecond\nthird one"
	cases := []struct {
		name      string
		keys      []string
		want      string
		cur       EditorPos
		selection string
	}{
		{"typing inserts at the cursor", []string{"# "}, "# first line\nsecond\nthird one", EditorPos{0, 2}, ""},
		{"enter splits the line", []string{"right", "right", "enter"}, "fi\nrst line\nsecond\nthird one", EditorPos{1, 0}, ""},
		{"backspace at a line's start joins it", []string{"down", "backspace"}, "first linesecond\nthird one", EditorPos{0, 10}, ""},
		{"delete at a line's end joins the next", []string{"end", "delete"}, "first linesecond\nthird one", EditorPos{0, 10}, ""},
		{"down keeps the column, clamped on a short line", []string{"end", "down", "down"}, text, EditorPos{2, 9}, ""},
		{"end and home of the line", []string{"down", "end"}, text, EditorPos{1, 6}, ""},
		{"ctrl+end and ctrl+home of the note", []string{"ctrl+end"}, text, EditorPos{2, 9}, ""},
		{"shift+right selects", []string{"shift+right", "shift+right"}, text, EditorPos{0, 2}, "fi"},
		{"shift+down selects across lines", []string{"right", "shift+down"}, text, EditorPos{1, 1}, "irst line\ns"},
		{"typing replaces the selection", []string{"shift+end", "X"}, "X\nsecond\nthird one", EditorPos{0, 1}, ""},
		{"backspace removes the selection", []string{"shift+down", "backspace"}, "second\nthird one", EditorPos{0, 0}, ""},
		{"a move without shift drops the selection", []string{"shift+right", "right"}, text, EditorPos{0, 2}, ""},
		{"ctrl+shift+end selects to the end", []string{"down", "ctrl+shift+end"}, text, EditorPos{2, 9}, "second\nthird one"},
		{"a pasted \\r\\n is one line break", []string{"a\r\nb"}, "a\nbfirst line\nsecond\nthird one", EditorPos{1, 1}, ""},
		{"tab types two spaces", []string{"tab"}, "  first line\nsecond\nthird one", EditorPos{0, 2}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewEditor(text)
			e.SetSize(40, 10)
			press(t, e, c.keys...)
			if got := e.Value(); got != c.want {
				t.Errorf("text %q, want %q", got, c.want)
			}
			if e.Cursor() != c.cur {
				t.Errorf("cursor %+v, want %+v", e.Cursor(), c.cur)
			}
			if got := e.Selection(); got != c.selection {
				t.Errorf("selection %q, want %q", got, c.selection)
			}
		})
	}
}

// A long line wraps under one number; ↓ walks its wrapped rows; the view is
// the area's size, the gutter numbered only where a line starts.
func TestEditorWrapAndView(t *testing.T) {
	e := NewEditor("short\n" + strings.Repeat("abcdefghij", 3) + "\nend")
	e.SetSize(16, 4) // gutter " 1 │ " is 5 wide: 11 columns of text
	rows := e.View(false)
	want := []string{" 1 │ short", " 2 │ abcdefghija", "   │ bcdefghijab", "   │ cdefghij"}
	if len(rows) != 4 {
		t.Fatalf("%d rows, want 4", len(rows))
	}
	for i, r := range rows {
		if got := strings.TrimRight(ansi.Strip(r), " "); got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
		if w := ansi.StringWidth(r); w != 16 {
			t.Errorf("row %d is %d wide, want 16", i, w)
		}
	}
	press(t, e, "down", "down", "down")
	if e.Cursor() != (EditorPos{1, 22}) {
		t.Errorf("↓ over wrapped rows: %+v, want line 1 col 22", e.Cursor())
	}
	press(t, e, "down") // onto the last line: the view scrolls to keep the cursor in it
	if got := strings.TrimRight(ansi.Strip(e.View(false)[3]), " "); got != " 3 │ end" {
		t.Errorf("after scrolling the last row is %q", got)
	}
}

// A drag selects what it covers, a click on a line number selects the line,
// and go-to-line puts the cursor at a line's start.
func TestEditorMouseAndGoto(t *testing.T) {
	e := NewEditor("first line\nsecond\nthird one")
	e.SetSize(40, 10)
	e.Press(5+1, 0) // after "f": the gutter is 5 wide
	e.Drag(5+3, 1)
	if !e.Release() || e.Selection() != "irst line\nsec" {
		t.Errorf("drag selected %q", e.Selection())
	}
	e.Press(1, 1)
	e.Release()
	if e.Selection() != "second\n" {
		t.Errorf("a click on a line number selected %q", e.Selection())
	}
	e.Press(5+2, 2)
	if e.Release() || e.HasSelection() {
		t.Error("a click without a drag must not leave a selection")
	}
	if !e.GotoLine(2) || e.Cursor() != (EditorPos{1, 0}) {
		t.Errorf("goto 2: %+v", e.Cursor())
	}
	if e.GotoLine(0) || e.GotoLine(4) {
		t.Error("a line that does not exist must be refused")
	}
}

// The cursor is drawn in the theme's cursor colour, and follows the theme.
func TestEditorCursorFollowsTheme(t *testing.T) {
	defer SetTheme(DefaultTheme())
	e := NewEditor("ab")
	e.SetSize(20, 1)
	cell := func() string { return strings.TrimPrefix(e.View(true)[0], StyleDim.Render(" 1 │ ")) }
	if got, want := cell(), StyleCursor.Render("a")+"b"; !strings.HasPrefix(got, want) {
		t.Errorf("cursor cell %q, want it drawn with StyleCursor %q", got, want)
	}
	th := DefaultTheme()
	th.Cursor = "201"
	SetTheme(th)
	if !strings.HasPrefix(cell(), StyleCursor.Render("a")) || StyleCursor.GetBackground() != th.Cursor {
		t.Error("a new theme's cursor colour is not used")
	}
}

// Option+←→ jump over words as Terminal.app sends them (Esc b, Esc f) and as
// iTerm does (alt+left, alt+right), Shift extending the selection; Option+⌫
// deletes the word before the cursor, and at a line's edge each goes on to
// the next line.
func TestEditorWords(t *testing.T) {
	const text = "hello, big world\nnext"
	altB := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: true}
	altF := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}, Alt: true}
	altLeft := tea.KeyMsg{Type: tea.KeyLeft, Alt: true}
	altRight := tea.KeyMsg{Type: tea.KeyRight, Alt: true}
	shiftAltLeft := tea.KeyMsg{Type: tea.KeyShiftLeft, Alt: true}
	altBack := tea.KeyMsg{Type: tea.KeyBackspace, Alt: true}
	end := tea.KeyMsg{Type: tea.KeyEnd}
	cases := []struct {
		name      string
		keys      []tea.KeyMsg
		want      string
		cur       EditorPos
		selection string
	}{
		{"esc f to the end of each word", []tea.KeyMsg{altF, altF}, text, EditorPos{0, 10}, ""},
		{"alt+right the same", []tea.KeyMsg{altRight}, text, EditorPos{0, 5}, ""},
		{"esc b from the end", []tea.KeyMsg{end, altB}, text, EditorPos{0, 11}, ""},
		{"alt+left over the comma", []tea.KeyMsg{end, altLeft, altLeft, altLeft}, text, EditorPos{0, 0}, ""},
		{"at a line's end the next line", []tea.KeyMsg{end, altF}, text, EditorPos{1, 0}, ""},
		{"shift+alt+left selects a word", []tea.KeyMsg{end, shiftAltLeft}, text, EditorPos{0, 11}, "world"},
		{"alt+backspace deletes a word", []tea.KeyMsg{end, altBack}, "hello, big \nnext", EditorPos{0, 11}, ""},
		{"alt+b f stay moves, not text", []tea.KeyMsg{altF, altB}, text, EditorPos{0, 0}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewEditor(text)
			e.SetSize(40, 10)
			for _, k := range c.keys {
				if !e.Key(k) {
					t.Fatalf("key %q not taken", k.String())
				}
			}
			if got := e.Value(); got != c.want {
				t.Errorf("text %q, want %q", got, c.want)
			}
			if e.Cursor() != c.cur {
				t.Errorf("cursor %+v, want %+v", e.Cursor(), c.cur)
			}
			if got := e.Selection(); got != c.selection {
				t.Errorf("selection %q, want %q", got, c.selection)
			}
		})
	}
}
