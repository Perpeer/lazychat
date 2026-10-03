package term

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// Claude sets the window title to "✳ <name>" while idle. The emulator's
// parser takes the 0x9C inside ✳'s UTF-8 for a string terminator and used
// to print the rest of the title where the cursor was: in the prompt.
func TestStringControls(t *testing.T) {
	s, err := Start(8, "title", "p", t.TempDir(), []string{"/bin/sh", "-c", `printf '\033[2;3H\033]0;\342\234\263 configuration\007\033]2;\303\234 x\033\\AB'; sleep 0.3`}, 40, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(stripANSI(s.Render()), "AB") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	row := strings.Split(stripANSI(s.Render()), "\n")[1]
	if got := strings.TrimSpace(row); got != "AB" {
		t.Errorf("row 2 shows %q, want only the text after the titles, AB", got)
	}
}

// However the reads split it, a title with 0x9C inside leaves no text, and
// text outside strings, UTF-8 included, is untouched.
func TestStringFilterSplits(t *testing.T) {
	seq := []byte("\x1b[1;1H\x1b]0;✳ configuration\a\x1b]2;Ü x\x1b\\✳ çÜ")
	for cut := 0; cut <= len(seq); cut++ {
		e := vt.NewSafeEmulator(30, 3)
		go func() {
			b := make([]byte, 256)
			for {
				if _, err := e.Read(b); err != nil {
					return
				}
			}
		}()
		var f stringFilter
		first, rest := append([]byte(nil), seq[:cut]...), append([]byte(nil), seq[cut:]...)
		f.apply(first)
		f.apply(rest)
		_, _ = e.Write(first)
		_, _ = e.Write(rest)
		row := ""
		for x := 0; x < 30; x++ {
			if c := e.CellAt(x, 0); c != nil {
				row += c.Content
			}
		}
		if got := strings.TrimSpace(row); got != "✳ çÜ" {
			t.Errorf("cut at %d: row %q, want the text after the titles", cut, got)
		}
	}
}

// A window title is read before the filter changes its bytes, split across
// reads too; other strings and a cancelled one set none.
func TestFilterTitle(t *testing.T) {
	var got []string
	f := stringFilter{onTitle: func(s string) { got = append(got, s) }}
	for _, chunk := range []string{"a\x1b]0;◐ Say", " hi\x07b", "\x1b]2;✳ done\x1b\\", "\x1bP1;x\x07", "\x1b]0;gone\x18", "\x1b]7;file://x\x07"} {
		f.apply([]byte(chunk))
	}
	if want := []string{"◐ Say hi", "✳ done"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("titles %q, want %q", got, want)
	}
}

// A program that leads its title with a spinner works; ✳ is done; one
// that sets no title works while its output is recent.
func TestWorking(t *testing.T) {
	s, err := Start(1, "t", "p", t.TempDir(), []string{"/bin/sh", "-c", `printf '\033]0;\342\227\220 x\007'; sleep 0.6; printf '\033]0;\342\234\263 x\007'; sleep 5`}, 80, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Kill()
	wait := func(want bool, why string) {
		t.Helper()
		for end := time.Now().Add(3 * time.Second); s.Working() != want; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(end) {
				t.Fatalf("%s: working %v, title %q", why, s.Working(), s.Title())
			}
		}
	}
	wait(true, "the spinner")
	wait(false, "✳")
	quiet, err := Start(2, "q", "p", t.TempDir(), []string{"/bin/sh", "-c", "echo hi; sleep 5"}, 80, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer quiet.Kill()
	for end := time.Now().Add(2 * time.Second); !quiet.Working(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("recent output without a title did not count as working")
		}
	}
}
