package kit

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"lazychat/internal/core/keylayout"
)

// The leave key is the one key lazychat keeps for itself while a program in
// a pane has the keys: Ctrl+Q, a single byte that every keyboard layout
// types the same way and no program here needs. Esc stays the program's:
// claude's answer stops on it, vim leaves insert mode with it.
const (
	esc   = 0x1b
	ctrlQ = 0x11
)

// leaveAt is the length of the leave key at data[i:], 0 when there is none.
// Under the kitty keyboard protocol, which the App turns on for claude,
// Ctrl+Q arrives as CSI 113;5 u; with Shift, Alt or Super held too it is
// another key, still not the program's.
func leaveAt(data []byte, i int) int {
	if data[i] == ctrlQ {
		return 1
	}
	rest := data[i:]
	if !bytes.HasPrefix(rest, []byte("\x1b[113;")) {
		return 0
	}
	end := bytes.IndexByte(rest, 'u')
	if end < 0 || end > 16 {
		return 0
	}
	_, mods, _ := strings.Cut(string(rest[2:end]), ";")
	mods, event, _ := strings.Cut(mods, ":")
	m, err := strconv.Atoi(mods)
	if err != nil {
		return 0
	}
	// Modifiers are 1 + a bit set, Ctrl being 4; Caps Lock (64) and Num Lock
	// (128) do not change the key. A release (event 3) is not a press.
	if (m-1)&^(64|128) != 4 || event == "3" {
		return 0
	}
	return end + 1
}

// InputRouter sits between the terminal and Bubble Tea. While a session has
// the focus every byte the terminal sends goes to that session's pty exactly
// as it arrived — Shift+Enter, Alt+Enter, kitty-protocol keys and whatever
// else claude understands — instead of being parsed into key names and
// re-encoded, which loses what the parser does not know. Three things are
// taken out: the tab keys, which go to the shell captured or not; the leave
// key, Ctrl+Q, which hands the keys back to the tab; and the mouse
// reports (SGR, ESC [ < b ; x ; y M|m), whose coordinates are the whole
// screen's: they go to the mouse callback, which moves them into the pane.
type InputRouter struct {
	src      io.Reader
	toTerm   atomic.Bool
	mu       sync.Mutex
	write    func([]byte)                       // the focused session's input
	leave    func()                             // runs when the leave key arrives
	mouse    func(code, x, y int, release bool) // x, y one-based, as the terminal sent them
	tab      func(n int)                        // runs when a tab key arrives, captured or not
	ch       chan []byte                        // what Bubble Tea gets to read
	buf      []byte
	pending  []byte      // an unfinished mouse report, completed by the next read
	escTimer *time.Timer // sends a lone Esc held in pending when no report follows
	last     atomic.Pointer[[]byte]
	option   func(rune) (rune, bool) // what Option types on the key that types a rune
	// CmdEnter runs when Cmd+Enter arrives while no session has the keys:
	// Bubble Tea v1 has no Cmd modifier, so it never sees the key itself.
	CmdEnter func()
}

// LastBytes is the last chunk read for Bubble Tea, as the terminal sent
// it, before any report in it was turned into its character: the key log shows it beside the key's name, which is what tells a
// key the parser does not know from one no binding takes.
func (r *InputRouter) LastBytes() []byte {
	if p := r.last.Load(); p != nil {
		return *p
	}
	return nil
}

// NewInputRouter reads src for Bubble Tea; tab is told of a tab key, ⌘1 to
// ⌘9, wherever the keys are going.
func NewInputRouter(src io.Reader, tab func(n int)) *InputRouter {
	r := &InputRouter{src: src, tab: tab, ch: make(chan []byte, 64), option: keylayout.Option}
	go r.loop()
	return r
}

// tabAt is the tab number and length of a tab key at data[i:], (0, 0) when
// there is none. ⌘1 to ⌘9 arrive as the kitty protocol spells them, CSI
// 49;9u to 57;9u (a digit with Super held): terminals that speak it send
// that for the keys, and install.sh makes iTerm send it. With Shift, Alt or
// Ctrl held too it is another key, and stays the program's.
func tabAt(data []byte, i int) (n, length int) {
	rest := data[i:]
	if !bytes.HasPrefix(rest, []byte("\x1b[")) {
		return 0, 0
	}
	end := bytes.IndexByte(rest, 'u')
	if end < 0 || end > 16 {
		return 0, 0
	}
	code, mods, _ := strings.Cut(string(rest[2:end]), ";")
	mods, _, _ = strings.Cut(mods, ":") // an event type after the modifiers
	c, err1 := strconv.Atoi(code)
	m, err2 := strconv.Atoi(mods)
	if err1 != nil || err2 != nil || c < '1' || c > '9' {
		return 0, 0
	}
	// Modifiers are 1 + a bit set; Caps Lock (64) and Num Lock (128) do not change the key.
	if (m-1)&^(64|128) != 8 {
		return 0, 0
	}
	return c - '0', end + 1
}

// cmdEnterAt is the length of a Cmd+Enter report at data[i:], 0 when there
// is none: CSI 13;9u in the kitty protocol (Enter with Super held), which a
// terminal that speaks it, or an iTerm key mapping, sends; Terminal.app
// keeps the key for itself.
func cmdEnterAt(data []byte, i int) int {
	for _, seq := range []string{"\x1b[13;9u", "\x1b[13;9:1u"} {
		if bytes.HasPrefix(data[i:], []byte(seq)) {
			return len(seq)
		}
	}
	return 0
}

// takeTabs calls the tab callback for every tab key in chunk and returns the
// rest, so Bubble Tea never sees them; a kitty report of a typed character,
// and Option sent as Meta, are passed on as the character typed.
func (r *InputRouter) takeTabs(chunk []byte) []byte {
	var out []byte
	for i := 0; i < len(chunk); {
		if n, l := tabAt(chunk, i); l > 0 {
			if r.tab != nil {
				r.tab(n)
			}
			i += l
			continue
		}
		if l := cmdEnterAt(chunk, i); l > 0 && r.CmdEnter != nil {
			r.CmdEnter()
			i += l
			continue
		}
		if text, l := textKey(chunk, i, r.option); l > 0 {
			out = append(out, text...)
			i += l
			continue
		}
		if text, l := metaKey(chunk, i, r.option); l > 0 {
			out = append(out, text...)
			i += l
			continue
		}
		out = append(out, chunk[i])
		i++
	}
	return out
}

// Focus routes the terminal's bytes to a session, or back to Bubble Tea when nil.
func (r *InputRouter) Focus(write func([]byte), leave func(), mouse func(code, x, y int, release bool)) {
	r.mu.Lock()
	r.write, r.leave, r.mouse = write, leave, mouse
	r.pending = nil
	r.mu.Unlock()
	r.toTerm.Store(write != nil)
}

func (r *InputRouter) loop() {
	b := make([]byte, 4096)
	for {
		n, err := r.src.Read(b)
		if n > 0 {
			chunk := append([]byte(nil), b[:n]...)
			if r.toTerm.Load() {
				r.toTerminal(chunk)
			} else if rest := r.takeTabs(chunk); len(rest) > 0 {
				r.last.Store(&chunk)
				r.ch <- rest
			}
		}
		if err != nil {
			close(r.ch)
			return
		}
	}
}

var mousePrefix = []byte("\x1b[<")

// escWait is how long a read ending in a lone Esc is held for the rest of a
// mouse report: a busy terminal cut "ESC [<65;67;49M" after the Esc and the
// rest reached claude as text, "<65;67;49M" in its prompt on every wheel
// step. A key's Esc comes on its own and goes on once the wait is over.
const escWait = 25 * time.Millisecond

// flushEsc sends a lone Esc held for a report that did not come.
func (r *InputRouter) flushEsc() {
	r.mu.Lock()
	held, write := r.pending, r.write
	if !bytes.Equal(held, []byte{esc}) {
		r.mu.Unlock()
		return
	}
	r.pending = nil
	r.mu.Unlock()
	if write != nil {
		write(held)
	}
}

func (r *InputRouter) toTerminal(chunk []byte) {
	r.mu.Lock()
	write, leave, mouse := r.write, r.leave, r.mouse
	data := append(r.pending, chunk...)
	r.pending = nil
	if r.escTimer != nil {
		r.escTimer.Stop()
	}
	r.mu.Unlock()

	var out []byte
	flush := func() {
		if len(out) > 0 && write != nil {
			write(out)
		}
		out = nil
	}
	for i := 0; i < len(data); {
		if n, l := tabAt(data, i); l > 0 {
			flush()
			if r.tab != nil {
				r.tab(n)
			}
			i += l
			continue
		}
		switch {
		case leaveAt(data, i) > 0:
			n := leaveAt(data, i)
			flush()
			r.toTerm.Store(false)
			if leave != nil {
				leave()
			}
			if rest := data[i+n:]; len(rest) > 0 {
				r.ch <- rest
			}
			return
		case bytes.HasPrefix(data[i:], mousePrefix):
			end := bytes.IndexAny(data[i+len(mousePrefix):], "Mm")
			if end < 0 {
				// A report cut in two by the read: keep it for the next one.
				flush()
				r.mu.Lock()
				r.pending = append([]byte(nil), data[i:]...)
				r.mu.Unlock()
				return
			}
			body := data[i+len(mousePrefix) : i+len(mousePrefix)+end+1]
			if f := strings.Split(string(body[:len(body)-1]), ";"); len(f) == 3 && mouse != nil {
				code, e1 := strconv.Atoi(f[0])
				x, e2 := strconv.Atoi(f[1])
				y, e3 := strconv.Atoi(f[2])
				if e1 == nil && e2 == nil && e3 == nil {
					mouse(code, x, y, body[len(body)-1] == 'm')
				}
			}
			i += len(mousePrefix) + end + 1
		case len(data)-i < len(mousePrefix) && bytes.HasPrefix(mousePrefix, data[i:]):
			// The read ended inside a report's start: hold it for the
			// next read, a lone Esc only for escWait.
			flush()
			r.mu.Lock()
			r.pending = append([]byte(nil), data[i:]...)
			if len(r.pending) == 1 {
				r.escTimer = time.AfterFunc(escWait, r.flushEsc)
			}
			r.mu.Unlock()
			return
		default:
			out = append(out, data[i])
			i++
		}
	}
	flush()
}

// Read is Bubble Tea's side: the bytes not claimed by a session.
func (r *InputRouter) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		c, ok := <-r.ch
		if !ok {
			return 0, io.EOF
		}
		r.buf = c
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// textKey is the text a kitty-protocol key report at data[i:] stands for,
// and the report's length; ("", 0) for anything else. The protocol can be
// left on by a program that ran in the terminal before lazychat, and then
// every key typed with a modifier — Option on a Mac layout, AltGr elsewhere —
// arrives as CSI code[:shifted] ; mods[:event] [; text] u, which Bubble Tea v1
// does not know and drops: brackets, @ and | would never reach the editor. The
// report is turned back into its characters, for any layout, when it carries
// text or when its only modifiers are Shift, Alt, Caps Lock and Num Lock;
// Ctrl, Super and a key release stay what they are. An Alt report without
// text becomes what Option types on that key, when option knows it.
func textKey(data []byte, i int, option func(rune) (rune, bool)) (string, int) {
	rest := data[i:]
	if !bytes.HasPrefix(rest, []byte("\x1b[")) {
		return "", 0
	}
	end := bytes.IndexByte(rest, 'u')
	if end < 0 || end > 64 || bytes.ContainsAny(rest[2:end], "\x1b<>=?") {
		return "", 0
	}
	fields := strings.Split(string(rest[2:end]), ";")
	keys := strings.Split(fields[0], ":")
	mods, event := 1, 1
	if len(fields) > 1 && fields[1] != "" {
		m, e, hasEvent := strings.Cut(fields[1], ":")
		var err error
		if mods, err = strconv.Atoi(m); err != nil {
			return "", 0
		}
		if hasEvent {
			if event, err = strconv.Atoi(e); err != nil {
				return "", 0
			}
		}
	}
	// Ctrl+Q, the way back to the list, is the byte Bubble Tea knows.
	if keys[0] == "113" && (mods-1)&^(64|128) == 4 && event != 3 {
		return string(rune(ctrlQ)), end + 1
	}
	if event == 3 || (mods-1)&^(1|2|64|128) != 0 {
		return "", 0
	}
	if len(fields) > 2 && fields[2] != "" {
		var b strings.Builder
		for _, cp := range strings.Split(fields[2], ":") {
			r, err := strconv.Atoi(cp)
			if err != nil || !printable(r) {
				return "", 0
			}
			b.WriteRune(rune(r))
		}
		return b.String(), end + 1
	}
	code, err := strconv.Atoi(keys[0])
	if err != nil {
		return "", 0
	}
	// Enter, Tab, Backspace and Esc come this way too under the protocol's
	// first flag; with no modifier they are the plain bytes, or Esc would
	// never leave the editor.
	if (code == 13 || code == 9 || code == 127 || code == 27) && (mods-1)&^(64|128) == 0 {
		return string(rune(code)), end + 1
	}
	shifted := false
	if (mods-1)&1 != 0 && len(keys) > 1 && keys[1] != "" {
		if c, err := strconv.Atoi(keys[1]); err == nil {
			code, shifted = c, true
		}
	}
	// With Shift held but no shifted key reported, which character the key
	// types is not known, so neither is its Option character.
	if (mods-1)&2 != 0 && option != nil && ((mods-1)&1 == 0 || shifted) {
		if o, ok := option(rune(code)); ok {
			code = int(o)
		}
	}
	if !printable(code) {
		return "", 0
	}
	return string(rune(code)), end + 1
}

// printable is a character the editor can hold: not a control code, and not in
// the private-use block where the protocol numbers keys like F1 and the arrows.
func printable(r int) bool {
	return r >= 0x20 && r != 0x7f && (r < 0xe000 || r > 0xf8ff) && r <= 0x10ffff
}

// metaKey is what Option typed when the terminal sends it as Meta: ESC and
// the character the key types without Option, which is all such a terminal
// tells, so the Option character comes from the system's layout. ("", 0)
// when data[i:] is not that or option does not know the key; ESC [ and
// ESC O start other keys' sequences, and ESC b and ESC f are what
// Terminal.app sends for Option+←→, so they stay the word keys.
func metaKey(data []byte, i int, option func(rune) (rune, bool)) (string, int) {
	if option == nil || data[i] != esc || i+1 >= len(data) {
		return "", 0
	}
	c, size := utf8.DecodeRune(data[i+1:])
	if c == utf8.RuneError || c == '[' || c == 'O' || c == 'b' || c == 'f' || !printable(int(c)) {
		return "", 0
	}
	o, ok := option(c)
	if !ok || !printable(int(o)) {
		return "", 0
	}
	return string(o), 1 + size
}
