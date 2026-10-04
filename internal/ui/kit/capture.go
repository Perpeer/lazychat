package kit

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/keylayout"
	"lazychat/internal/core/sound"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Capture is a program in a pane holding the keys: Chat's sessions and
// Terminal's shells both take them through it, so they behave the same. While
// it holds them the input router hands every byte to the program's pty; the
// leave key, or a left click beside the pane, gives them back. The kitty
// keyboard mode the program asks for is relayed to the real terminal while
// it has the keys and dropped when it gives them up.
type Capture struct {
	screen Screen
	pane   *TermPane
	area   func() Rect // where the pane is on the screen now
	held   bool
	key    string // the program's pane key, so a late report from another is ignored

	// Left runs when the keys come back, before a click beside the pane is
	// handled as a click on the list.
	Left func()
	// Beside is a left click beside the pane while it held the keys, at a
	// screen cell: the tab puts its cursor there, without opening anything.
	Beside func(msg tea.MouseMsg)
	// HeldNewline makes Shift+Enter and Option+Enter the program's new line
	// where the terminal sends them as a plain Enter (Terminal.app without
	// Option as Meta): claude's sessions want it, an editor does not.
	HeldNewline bool
	// heldKeys is what is held on the keyboard now; tests stand it in.
	heldKeys func() (shift, option bool)
}

// The messages a Capture sends itself from the router's and the emulator's
// goroutines; each names its Capture, since every tab sees every message.
type (
	captureLeave struct{ c *Capture }
	captureTyped struct{ c *Capture }
	captureKitty struct {
		c     *Capture
		key   string
		flags int
	}
)

func NewCapture(screen Screen, pane *TermPane, area func() Rect) *Capture {
	return &Capture{screen: screen, pane: pane, area: area, heldKeys: keylayout.Held}
}

// newline is what the program gets for bytes the terminal sent. A read that
// is one Enter alone, with Shift or Option held as it arrives, is that key:
// the terminal sent all three the same, and macOS still knows which it was.
// A read with more in it is typing or a paste and passes as it came.
func (c *Capture) newline(b []byte, kitty int) []byte {
	if !c.HeldNewline || len(b) != 1 || b[0] != '\r' {
		return b
	}
	shift, option := c.heldKeys()
	switch {
	case shift && kitty != 0:
		return []byte("\x1b[13;2u")
	case option && kitty != 0:
		return []byte("\x1b[13;3u")
	case shift || option:
		return []byte("\x1b\r")
	}
	return b
}

func (c *Capture) Held() bool { return c.held }

// Take gives the pane's program the keys; false when there is no running
// program to give them to.
func (c *Capture) Take() bool {
	s := c.pane.Session
	if s == nil || !s.Alive() {
		return false
	}
	c.held, c.key = true, c.pane.Key
	c.screen.Capture(
		func(b []byte) {
			_ = s.Write(c.newline(b, s.Kitty()))
			c.screen.Send(captureTyped{c})
			if key := TypedKey(b); key != "" {
				c.screen.Send(PlaySound{Name: key})
			}
		},
		func() { c.screen.Send(captureLeave{c}) },
	)
	key := c.key
	// Sent from a new goroutine: the hook runs inside the emulator's write,
	// under its lock, and the loop that must receive may be drawing that screen.
	s.OnKitty(func(flags int) { go c.screen.Send(captureKitty{c: c, key: key, flags: flags}) })
	c.screen.SetKitty(s.Kitty())
	s.Focus(true)
	return true
}

// Drop takes the keys back; nothing happens when they were not held.
func (c *Capture) Drop() {
	if !c.held {
		return
	}
	c.held = false
	if s := c.pane.Session; s != nil {
		s.Focus(false)
	}
	c.screen.SetKitty(0)
	c.screen.Release()
	if c.Left != nil {
		c.Left()
	}
}

// Update takes the Capture's own messages and, while it holds the keys, the
// mouse reports the router lifts out of the program's input; false for
// anything else, which the tab handles.
func (c *Capture) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case captureLeave:
		if msg.c == c {
			c.Drop()
		}
		return msg.c == c
	case captureTyped:
		if msg.c == c {
			c.pane.ToLive()
		}
		return msg.c == c
	case captureKitty:
		if msg.c == c && c.held && c.pane.Key == msg.key {
			c.screen.SetKitty(msg.flags)
		}
		return msg.c == c
	case RawMouse:
		if !c.held {
			return false
		}
		c.rawMouse(msg)
		return true
	}
	return false
}

// rawMouse is a mouse report from the program's input, one-based as the
// terminal sent it. Inside the pane, and any drag, release or wheel, it is
// the program's. A left click beside the pane is the way back that works in
// every terminal, whatever keys it can send.
func (c *Capture) rawMouse(m RawMouse) {
	x, y := m.X-1, m.Y-1
	r := c.area()
	inside := x >= r.X0 && y >= r.Y0 && x < r.X0+r.Cols && y < r.Y0+r.Rows
	if inside || m.Release || m.Code&^(4|8|16) != 0 {
		c.pane.Mouse(r, m.Code, x, y, m.Release)
		return
	}
	c.Drop()
	if c.Beside != nil {
		c.Beside(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	}
}

// TypedKey is the key sound for bytes sent to a pane when they are a key a
// person typed into text — a character, the space bar, Enter, Backspace,
// plain or as a kitty CSI u report; "" for an arrow, a mouse report or a
// paste.
func TypedKey(b []byte) sound.Name {
	code := -1
	switch {
	case len(b) == 0:
	case b[0] != 0x1b:
		if utf8.RuneCount(b) == 1 {
			r, _ := utf8.DecodeRune(b)
			code = int(r)
		}
	case len(b) > 3 && b[1] == '[' && b[len(b)-1] == 'u':
		c, _, _ := strings.Cut(string(b[2:len(b)-1]), ";")
		c, _, _ = strings.Cut(c, ":")
		if n, err := strconv.Atoi(c); err == nil {
			code = n
		}
	}
	switch {
	case code == ' ':
		return sound.KeySpace
	case code == '\r' || code == '\n':
		return sound.KeyEnter
	case code == 0x7f || code == 8:
		return sound.KeyBackspace
	case code == '\t' || code > ' ' && code < 57344:
		return sound.Key
	}
	return ""
}
