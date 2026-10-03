package kit

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// The router hands a focused session every byte as it came, Esc included,
// keeps only the leave key, and gives Bubble Tea what follows it.
func TestInputRouter(t *testing.T) {
	pr, pw := io.Pipe()
	r := NewInputRouter(pr, nil)
	var got bytes.Buffer
	left := make(chan struct{}, 1)
	r.Focus(func(b []byte) { got.Write(b) }, func() { left <- struct{}{} }, nil)
	_, _ = pw.Write([]byte("m\x1b[13;2u\x1b\r"))
	_, _ = pw.Write([]byte("x\x1b"))
	time.Sleep(20 * time.Millisecond)
	_, _ = pw.Write([]byte("\x11q"))
	select {
	case <-left:
	case <-time.After(2 * time.Second):
		t.Fatal("leave never fired")
	}
	if got.String() != "m\x1b[13;2u\x1b\rx\x1b" {
		t.Errorf("session got %q", got.String())
	}
	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil || string(buf[:n]) != "q" {
		t.Errorf("Bubble Tea got %q, %v; want what came after the leave key", buf[:n], err)
	}
	pw.Close()
	if _, err := r.Read(buf); err != io.EOF {
		t.Errorf("after the source closes Read must return EOF, got %v", err)
	}
}

// Mouse reports never reach the session as typed bytes: each becomes a
// callback, and a report cut across two reads is joined first.
func TestRouterMouse(t *testing.T) {
	pr, pw := io.Pipe()
	r := NewInputRouter(pr, nil)
	var mu sync.Mutex
	var got bytes.Buffer
	var codes []int
	r.Focus(func(b []byte) { mu.Lock(); got.Write(b); mu.Unlock() }, nil, func(code, x, y int, release bool) {
		mu.Lock()
		codes = append(codes, code)
		mu.Unlock()
	})
	for _, chunk := range []string{"a\x1b[<64;80;10M", "b\x1b[<0;5;5Mc\x1b[<0;5;5m", "\x1b[<6", "5;80;10Md", "\x1b[<80;1;1M\x1bx"} {
		_, _ = pw.Write([]byte(chunk))
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if got.String() != "abcd\x1bx" {
		t.Errorf("session got %q; want the typed bytes and Alt+x, no mouse reports", got.String())
	}
	if want := []int{64, 0, 0, 65, 80}; fmt.Sprint(codes) != fmt.Sprint(want) {
		t.Errorf("mouse codes = %v; want %v (wheel, press, release, split wheel, ctrl wheel)", codes, want)
	}
	pw.Close()
}

func TestListWindow(t *testing.T) {
	var l List
	l.Sel = 7
	if start, end := l.Window(20, 5, 1); start != 3 || end != 8 {
		t.Errorf("window keeps the cursor visible: %d..%d", start, end)
	}
	l.Sel = 0
	if start, end := l.Window(20, 10, 4); start != 0 || end != 2 {
		t.Errorf("four-row items, ten rows: %d..%d", start, end)
	}
	l.Move(-5, 20)
	if l.Sel != 0 {
		t.Errorf("move clamps at the top: %d", l.Sel)
	}
}

// The leave key, Ctrl+Q, in every form a terminal sends it, and near misses
// that are other keys and must reach the program: Esc above all.
func TestLeaveAt(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		leave bool // the whole input is the leave key
	}{
		{"Ctrl+Q", "\x11", true},
		{"kitty Ctrl+Q", "\x1b[113;5u", true},
		{"kitty Ctrl+Q with an event type", "\x1b[113;5:1u", true},
		{"kitty Ctrl+Q with Caps Lock on", "\x1b[113;69u", true},
		{"kitty Ctrl+Q released is not a press", "\x1b[113;5:3u", false},
		{"kitty Ctrl+Shift+Q is another key", "\x1b[113;6u", false},
		{"kitty q is typed", "\x1b[113u", false},
		{"Esc alone is the program's", "\x1b", false},
		{"kitty Esc is the program's", "\x1b[27u", false},
		{"Alt+q", "\x1bq", false},
		{"Ctrl+C is not the leave key", "\x03", false},
		{"iTerm's old ⌘⌫, 0x1c, is not", "\x1c", false},
		{"kitty Ctrl+C is not", "\x1b[99;5u", false},
		{"kitty Shift+Enter is the program's", "\x1b[13;2u", false},
		{"a plain q", "q", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := 0
			if c.leave {
				want = len(c.in)
			}
			if got := leaveAt([]byte(c.in), 0); got != want {
				t.Errorf("leaveAt(%q) = %d, want %d", c.in, got, want)
			}
		})
	}
}

// ⌘1 to ⌘9 in the kitty form reach the shell whether a session has the keys
// or not; the same digits with other modifiers are the program's.
func TestTabKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		tab  int // 0 when the input is not a tab key
	}{
		{"⌘1", "\x1b[49;9u", 1},
		{"⌘2", "\x1b[50;9u", 2},
		{"⌘2 with an event type", "\x1b[50;9:1u", 2},
		{"⌘2 with Caps Lock on", "\x1b[50;73u", 2},
		{"⌘9", "\x1b[57;9u", 9},
		{"⌘0 is no tab", "\x1b[48;9u", 0},
		{"⌘⇧1 is the program's", "\x1b[49;10u", 0},
		{"Ctrl+1 is the program's", "\x1b[49;5u", 0},
		{"a plain 1", "1", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, l := tabAt([]byte(c.in), 0)
			if n != c.tab || (c.tab > 0) != (l == len(c.in)) {
				t.Errorf("tabAt(%q) = %d, %d; want tab %d", c.in, n, l, c.tab)
			}
		})
	}
	// Taken out of the stream in both modes, the rest passes on.
	var got []int
	r := &InputRouter{tab: func(n int) { got = append(got, n) }}
	if rest := r.takeTabs([]byte("a\x1b[50;9ub")); string(rest) != "ab" || len(got) != 1 || got[0] != 2 {
		t.Errorf("uncaptured: rest %q, tabs %v", rest, got)
	}
	var typed []byte
	r.write = func(b []byte) { typed = append(typed, b...) }
	r.toTerminal([]byte("x\x1b[49;9uy"))
	if string(typed) != "xy" || len(got) != 2 || got[1] != 1 {
		t.Errorf("captured: session got %q, tabs %v", typed, got)
	}
}

// A kitty report of a typed character becomes the character; anything else
// is left alone.
func TestTextKey(t *testing.T) {
	cases := []struct {
		in, want string
		length   int
	}{
		{"\x1b[91u", "[", 5},       // a bare report of '['
		{"\x1b[56;3;91u", "[", 10}, // Option+8 carrying its text
		{"\x1b[233;1u", "é", 8},    // an accented letter by code point
		{"\x1b[97:65;2u", "A", 10}, // Shift: the shifted key
		{"\x1b[124;1:2u", "|", 10}, // a repeat is typed again
		{"\x1b[27u", "\x1b", 5},    // Esc, so the editor can be left
		{"\x1b[13u", "\r", 5},      // Enter
		{"\x1b[99;5u", "", 0},      // Ctrl+C is a key, not text
		{"\x1b[49;9u", "", 0},      // ⌘1 is a tab key
		{"\x1b[91;1:3u", "", 0},    // a release types nothing
		{"\x1b[57399u", "", 0},     // a functional key in the private block
		{"\x1b[13;2u", "", 0},      // Shift+Enter stays a key
		{"\x1b[Au", "", 0},         // an arrow followed by a typed u
		{"\x1b[<0;10;5M", "", 0},   // a mouse report
		{"\x1b[113;5u", "\x11", 8}, // ctrl+q, the way back to the list
		{"\x1b[113;5:3u", "", 0},   // its release
		{"plain", "", 0},
	}
	for _, c := range cases {
		got, n := textKey([]byte(c.in), 0, nil)
		if got != c.want || n != c.length {
			t.Errorf("textKey(%q) = %q, %d; want %q, %d", c.in, got, n, c.want, c.length)
		}
	}
}

// Option sent as Meta, and an Alt report without text, type what Option
// types on that key in the layout; a session gets the bytes as they came,
// and a key the layout has nothing for stays what it was.
func TestOptionAsMeta(t *testing.T) {
	turkish := map[rune]rune{'9': ']', '8': '[', ')': 'Ø', 'q': '@', 'b': '∫', 'f': 'ƒ'}
	option := func(r rune) (rune, bool) { o, ok := turkish[r]; return o, ok }
	r := &InputRouter{option: option}
	for in, want := range map[string]string{
		"\x1b9":         "]",
		"a\x1b8b":       "a[b",
		"\x1b)":         "Ø",
		"\x1bj":         "\x1bj", // not in the layout: Alt+J as before
		"\x1bb":         "\x1bb", // Terminal.app's Option+←: a word key, not ∫
		"\x1bf":         "\x1bf",
		"\x1b[A":        "\x1b[A",
		"\x1b":          "\x1b",
		"\x1b[57;3u":    "]", // kitty, Alt+9 without text
		"\x1b[57;3;93u": "]",
		"\x1b[57;4u":    "9", // Shift+Alt with no shifted key: unknown, as before
		"\x1b[41:41;4u": "Ø",
	} {
		if got := string(r.takeTabs([]byte(in))); got != want {
			t.Errorf("uncaptured %q → %q, want %q", in, got, want)
		}
	}
	var typed []byte
	r.write = func(b []byte) { typed = append(typed, b...) }
	r.toTerminal([]byte("\x1b9\x1b\r"))
	if string(typed) != "\x1b9\x1b\r" {
		t.Errorf("a session got %q, want Option+9 and Option+Enter as sent", typed)
	}
}
