package term

import (
	"strings"
	"testing"
	"time"
)

// A real pty and the emulator together: the program's output shows up on the
// rendered screen, its exit is seen, and a stop ends what is still running.
func TestSessionLifecycle(t *testing.T) {
	woke := make(chan struct{}, 64)
	s, err := Start(1, "t", "p", t.TempDir(), []string{"/bin/sh", "-c", "printf 'hello-from-pty'; sleep 0.2"}, 40, 10,
		func() {
			select {
			case woke <- struct{}{}:
			default:
			}
		})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if strings.Contains(stripANSI(s.Render()), "hello-from-pty") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("output never reached the screen:\n%s", s.Render())
		}
		<-woke
	}
	deadline = time.Now().Add(3 * time.Second)
	for s.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if done, err := s.Exit(); !done || err != nil {
		t.Errorf("exit: done=%v err=%v", done, err)
	}
	if err := s.Write([]byte("x")); err == nil {
		t.Error("writing to an ended session must fail")
	}

	long, err := Start(2, "long", "p", t.TempDir(), []string{"/bin/sh", "-c", "sleep 30"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	StopAll([]*Session{long}, 2*time.Second)
	if long.Alive() {
		t.Error("StopAll left the process running")
	}
}

func TestChildEnv(t *testing.T) {
	got := childEnv([]string{"HOME=/h", "CLAUDECODE=1", "CLAUDE_CODE_CHILD_SESSION=1", "TERM=dumb", "CLAUDE_CODE_USE_BEDROCK=1", "LAZYCHAT_TRACE=/tmp/t"})
	want := []string{"HOME=/h", "CLAUDE_CODE_USE_BEDROCK=1", "TERM=xterm-256color", "COLORTERM=truecolor"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("childEnv = %v, want %v", got, want)
	}
}

// The program asks for the kitty keyboard protocol and queries it back: the
// session remembers the flags and answers the query into the pty, where the
// cooked tty echoes it (^[[?1u) onto the screen; a pop restores the flags.
func TestKittyRelay(t *testing.T) {
	s, err := Start(4, "k", "p", t.TempDir(), []string{"/bin/sh", "-c", `printf '\033[>1u\033[?u'; sleep 0.3; printf '\033[<u'; sleep 0.2`}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	sawFlag := false
	for s.Alive() && time.Now().Before(deadline) {
		if s.Kitty() == 1 {
			sawFlag = true
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !sawFlag {
		t.Error("the push never set the flags to 1")
	}
	if screen := stripANSI(s.Render()); !strings.Contains(screen, "[?1u") {
		t.Errorf("the query was not answered with CSI ? 1 u:\n%s", screen)
	}
	if s.Kitty() != 0 {
		t.Errorf("after the pop the flags should be 0, got %d", s.Kitty())
	}
}

// What leaves the top of the screen stays readable: the history is the
// scrollback then the live screen, with stable indices.
func TestHistory(t *testing.T) {
	s, err := Start(5, "h", "p", t.TempDir(), []string{"/bin/sh", "-c", "i=1; while [ $i -le 100 ]; do printf 'line %03d\\n' $i; i=$((i+1)); done; sleep 0.3"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for s.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	total := s.Total()
	if total < 100 {
		t.Fatalf("history holds %d rows, want at least 100", total)
	}
	_, plain := s.Rows(0, 3)
	if len(plain) != 3 || plain[0] != "line 001" || plain[2] != "line 003" {
		t.Errorf("oldest rows = %q", plain)
	}
	styled, _ := s.Rows(total-10, total)
	if len(styled) != 10 {
		t.Errorf("the last window has %d rows", len(styled))
	}
}

// A program that turned mouse tracking on gets wheel events in its own
// encoding; one that did not gets nothing.
func TestMouse(t *testing.T) {
	s, err := Start(6, "m", "p", t.TempDir(), []string{"/bin/sh", "-c", `stty raw -echo; printf '\033[?1049h\033[?1000h\033[?1006h'; head -c 10 | od -An -c; sleep 0.3`}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The alternate screen is set in the same write that turns tracking on.
	for end := time.Now().Add(3 * time.Second); !s.AltScreen() && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	s.Mouse(64, 4, 2, false)
	deadline := time.Now().Add(3 * time.Second)
	for s.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if screen := strings.Join(strings.Fields(stripANSI(s.Render())), " "); !strings.Contains(screen, "[ < 6 4 ; 5 ; 3 M") {
		t.Errorf("the program did not get the wheel as SGR at (5,3):\n%s", screen)
	}
}

func TestResizeClamps(t *testing.T) {
	s, err := Start(3, "r", "p", t.TempDir(), []string{"/bin/sh", "-c", "sleep 0.3"}, 5, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Kill()
	if c, r := s.Size(); c != 20 || r != 5 {
		t.Errorf("size clamped to %dx%d, want 20x5", c, r)
	}
	s.Resize(100, 30)
	if c, r := s.Size(); c != 100 || r != 30 {
		t.Errorf("resize gave %dx%d", c, r)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case esc:
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				esc = false
			}
		case r == 0x1b:
			esc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Resizing keeps an inline program's text anchored at the bottom, as a
// terminal does: taller brings lines back from the scrollback, shorter moves
// the top rows into it, and the prompt stays on the last row either way.
func TestResizeKeepsTheBottom(t *testing.T) {
	s, err := Start(1, "t", "p", t.TempDir(), []string{"/bin/sh", "-c", "i=1; while [ $i -le 40 ]; do echo line $i; i=$((i+1)); done; printf PROMPT; sleep 30"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	rows := func() []string { return strings.Split(stripANSI(s.Render()), "\n") }
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(stripANSI(s.Render()), "PROMPT") {
		if time.Now().After(deadline) {
			t.Fatalf("no prompt:\n%s", s.Render())
		}
		time.Sleep(10 * time.Millisecond)
	}
	cases := []struct {
		rows        int
		first, last string
	}{
		{20, "line 22", "PROMPT"},
		{6, "line 36", "PROMPT"},
		{10, "line 32", "PROMPT"},
	}
	for _, c := range cases {
		s.Resize(40, c.rows)
		got := rows()
		if len(got) != c.rows || !strings.HasPrefix(got[0], c.first) || !strings.HasPrefix(got[c.rows-1], c.last) {
			t.Errorf("at %d rows: first %q, last %q (%d rows), want %q … %q", c.rows, got[0], got[len(got)-1], len(got), c.first, c.last)
		}
		if _, y, _ := s.Cursor(); y != c.rows-1 {
			t.Errorf("at %d rows the cursor is on row %d, want the last", c.rows, y)
		}
	}
	if n := s.emu.ScrollbackLen(); n != 31 {
		t.Errorf("scrollback has %d lines, want the 31 above the screen", n)
	}
}
