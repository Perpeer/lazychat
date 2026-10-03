package kit

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/term"
)

// capScreen records what a Capture asks of the screen.
type capScreen struct {
	write    func([]byte)
	leave    func()
	released int
	kitty    []int
	sent     []tea.Msg
}

func (s *capScreen) Push(Overlay)             {}
func (s *capScreen) Note(string, ...any)      {}
func (s *capScreen) Queue(tea.Cmd)            {}
func (s *capScreen) Send(m tea.Msg)           { s.sent = append(s.sent, m) }
func (s *capScreen) Release()                 { s.released++; s.write, s.leave = nil, nil }
func (s *capScreen) SetKitty(f int)           { s.kitty = append(s.kitty, f) }
func (s *capScreen) Size() (int, int)         { return 80, 24 }
func (s *capScreen) Header(string) string     { return "" }
func (s *capScreen) FooterLine([]Hint) string { return "" }
func (s *capScreen) Quit() tea.Cmd            { return nil }
func (s *capScreen) Switch(string)            {}
func (s *capScreen) Capture(w func([]byte), l func()) {
	s.write, s.leave = w, l
}

var _ Screen = (*capScreen)(nil)

// A running program takes the keys and gets the bytes typed; the leave key
// gives them back, and only its own Capture answers its messages. A click
// beside the pane gives them back too and becomes a click on the list; a
// report inside the pane stays the program's.
func TestCapture(t *testing.T) {
	s, err := term.Start(1, "cat", "p", t.TempDir(), []string{"/bin/cat"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	scr := &capScreen{}
	pane := &TermPane{}
	area := Rect{X0: 30, Y0: 1, Cols: 40, Rows: 10}
	c := NewCapture(scr, pane, func() Rect { return area })
	left, beside := 0, tea.MouseMsg{}
	c.Left = func() { left++ }
	c.Beside = func(m tea.MouseMsg) { beside = m }

	if c.Take() || c.Held() {
		t.Fatal("no program in the pane: nothing to take the keys")
	}
	pane.Point("k1", s, 40, 10)
	if !c.Take() || !c.Held() || scr.write == nil {
		t.Fatal("a running program should take the keys")
	}
	scr.write([]byte("hello\r"))
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(s.Render(), "hello") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(s.Render(), "hello") {
		t.Errorf("typed bytes did not reach the program:\n%s", s.Render())
	}
	other := NewCapture(scr, &TermPane{}, func() Rect { return area })
	scr.leave()
	leaveMsg := scr.sent[len(scr.sent)-1]
	if other.Update(leaveMsg) {
		t.Error("another tab's Capture took this one's leave key")
	}
	if !c.Update(leaveMsg) || c.Held() || left != 1 || scr.kitty[len(scr.kitty)-1] != 0 {
		t.Errorf("the leave key should give the keys back: held %v, left %d, kitty %v", c.Held(), left, scr.kitty)
	}

	c.Take()
	if !c.Update(RawMouse{Code: 0, X: 40, Y: 5}) || !c.Held() {
		t.Error("a click inside the pane is the program's")
	}
	if !c.Update(RawMouse{Code: 0, X: 5, Y: 3}) || c.Held() || left != 2 || beside.X != 4 || beside.Y != 2 {
		t.Errorf("a click beside should leave and land on the list: held %v, left %d, beside %+v", c.Held(), left, beside)
	}
	if c.Update(RawMouse{Code: 0, X: 5, Y: 3}) {
		t.Error("with the keys back, a raw report is not the Capture's")
	}
}

// A lone Enter with Shift or Option held is claude's new line: its kitty
// key when it asked for kitty, ESC CR otherwise; plain Enter, typing, a
// paste and an editor's capture pass as they came.
func TestHeldNewline(t *testing.T) {
	var shift, option bool
	c := &Capture{HeldNewline: true, heldKeys: func() (bool, bool) { return shift, option }}
	cases := []struct {
		name          string
		in            string
		shift, option bool
		kitty         int
		want          string
	}{
		{"Shift+Enter, kitty", "\r", true, false, 1, "\x1b[13;2u"},
		{"Option+Enter, kitty", "\r", false, true, 1, "\x1b[13;3u"},
		{"Shift+Enter, no kitty", "\r", true, false, 0, "\x1b\r"},
		{"Option+Enter, no kitty", "\r", false, true, 0, "\x1b\r"},
		{"Enter", "\r", false, false, 1, "\r"},
		{"typing that ends in Enter", "a\r", true, false, 1, "a\r"},
		{"a paste", "\x1b[200~x\ry\x1b[201~", true, false, 1, "\x1b[200~x\ry\x1b[201~"},
		{"Option+Enter as Meta already", "\x1b\r", false, true, 1, "\x1b\r"},
		{"Shift+Enter a terminal told apart", "\x1b[13;2u", true, false, 1, "\x1b[13;2u"},
	}
	for _, k := range cases {
		shift, option = k.shift, k.option
		if got := string(c.newline([]byte(k.in), k.kitty)); got != k.want {
			t.Errorf("%s: %q, want %q", k.name, got, k.want)
		}
	}
	editor := &Capture{heldKeys: func() (bool, bool) { return true, true }}
	if got := string(editor.newline([]byte("\r"), 1)); got != "\r" {
		t.Errorf("an editor's Enter became %q", got)
	}
}
