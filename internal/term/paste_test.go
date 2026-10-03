package term

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A program that asks for bracketed paste gets the text wrapped and nothing
// after it, even while it prints all the time; one that never asks gets
// nothing and ErrNoPaste at the limit.
func TestPasteReady(t *testing.T) {
	busy, err := Start(1, "busy", "p", t.TempDir(), []string{"/bin/sh", "-c", `stty raw -echo; printf '\033[?2004h> '; (while :; do printf .; sleep 0.05; done) & head -c 15 | od -An -tx1 | tr -d ' \n'; printf '<END>'; kill $!; sleep 1`}, 80, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := <-busy.PasteWhenReady("a\nb", 3*time.Second); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("a busy program waited %s for its paste", time.Since(start))
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(stripANSI(busy.Render()), "<END>") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	// ESC [ 2 0 0 ~ a LF b ESC [ 2 0 1 ~, as hex, and no CR after it.
	if got := strings.ReplaceAll(stripANSI(busy.Render()), ".", ""); !strings.Contains(strings.ReplaceAll(got, "\n", ""), "1b5b3230307e610a621b5b3230317e<END>") {
		t.Errorf("the program read:\n%s", got)
	}

	never, err := Start(2, "never", "p", t.TempDir(), []string{"/bin/sh", "-c", "printf 'no paste mode'; sleep 2"}, 80, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	if err := <-never.PasteWhenReady("x", 300*time.Millisecond); !errors.Is(err, ErrNoPaste) || time.Since(start) < 300*time.Millisecond {
		t.Errorf("never ready: %v after %s", err, time.Since(start))
	}
	_ = busy.Kill()
	_ = never.Kill()
}
