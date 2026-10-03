package term

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// With LAZYCHAT_TRACE set a session records its output as read and its
// other events, each after the count of output bytes before it.
func TestTrace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(traceEnv, dir)
	s, err := Start(7, "traced", "p", t.TempDir(), []string{"/bin/sh", "-c", "stty raw -echo; printf 'hello'; head -c 1 >/dev/null; sleep 0.2"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(stripANSI(s.Render()), "hello") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	s.Resize(50, 12)
	if err := s.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	for s.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	outs, _ := filepath.Glob(filepath.Join(dir, "*-7-traced.out"))
	if len(outs) != 1 {
		t.Fatalf("recordings: %v", outs)
	}
	if b, _ := os.ReadFile(outs[0]); string(b) != "hello" {
		t.Errorf("output %q, want hello", b)
	}
	ev, _ := os.ReadFile(strings.TrimSuffix(outs[0], ".out") + ".events")
	for _, want := range []string{"0 ", "start 40 10", "5 ", "resize 50 12", `input "x"`, "exit"} {
		if !strings.Contains(string(ev), want) {
			t.Errorf("events lack %q:\n%s", want, ev)
		}
	}
}

// TestReplay plays a recording back into the emulator the way a session
// would, the string filter and resizes included, and stops at the first
// point where the prompt row (the one opening with ❯) shows the needle: the
// bytes just before it are what the emulator took differently from the
// program's intent. What stays on screen is caught however coarse the steps.
//
// LAZYCHAT_REPLAY_RAW=1 leaves the string filter out, to see the recording
// as the emulator took it before the filter.
//
//	LAZYCHAT_REPLAY=<dir>/<stem> [LAZYCHAT_REPLAY_NEEDLE=configuration] [LAZYCHAT_REPLAY_RAW=1] go test ./internal/term -run TestReplay -v
func TestReplay(t *testing.T) {
	stem := os.Getenv("LAZYCHAT_REPLAY")
	if stem == "" {
		t.Skip("set LAZYCHAT_REPLAY to a recording's path without its extension")
	}
	needle := os.Getenv("LAZYCHAT_REPLAY_NEEDLE")
	if needle == "" {
		needle = "configuration"
	}
	out, err := os.ReadFile(stem + ".out")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("LAZYCHAT_REPLAY_RAW") == "" {
		var filter stringFilter
		filter.apply(out)
	}
	type resize struct {
		at         int64
		cols, rows int
	}
	var resizes []resize
	cols, rows := 80, 24
	f, err := os.Open(stem + ".events")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		at, _ := strconv.ParseInt(fields[0], 10, 64)
		c, _ := strconv.Atoi(fields[3])
		r, _ := strconv.Atoi(fields[4])
		switch fields[2] {
		case "start":
			cols, rows = c, r
		case "resize":
			resizes = append(resizes, resize{at, c, r})
		}
	}
	s := &Session{emu: vt.NewSafeEmulator(cols, rows), cols: cols, rows: rows}
	s.emu.SetScrollbackSize(scrollbackRows)
	// The emulator's answers to the program's queries go into a pipe; unread,
	// the first one blocks every write after it.
	go func() {
		b := make([]byte, 4096)
		for {
			if _, err := s.emu.Read(b); err != nil {
				return
			}
		}
	}()
	promptShows := func() (string, bool) {
		for _, row := range strings.Split(stripANSI(s.emu.Render()), "\n") {
			if l := strings.TrimSpace(row); strings.HasPrefix(l, "❯") && strings.Contains(l, needle) {
				return row, true
			}
		}
		return "", false
	}
	const step = 1024
	var pos int64
	for pos < int64(len(out)) {
		end := min(pos+step, int64(len(out)))
		for len(resizes) > 0 && resizes[0].at <= end {
			r := resizes[0]
			resizes = resizes[1:]
			if r.at > pos {
				_, _ = s.emu.Write(out[pos:r.at])
				pos = r.at
			}
			t.Logf("resize to %dx%d at byte %d", r.cols, r.rows, r.at)
			old := s.rows
			s.cols, s.rows = r.cols, r.rows
			s.resizeAnchored(old, r.cols, r.rows)
		}
		_, _ = s.emu.Write(out[pos:end])
		pos = end
		if row, ok := promptShows(); ok {
			from := max(0, pos-1500)
			t.Fatalf("the prompt row shows %q after byte %d:\n%s\nthe bytes before it:\n%s", needle, pos, row, fmt.Sprintf("%q", out[from:pos]))
		}
	}
	t.Logf("played %d bytes; the prompt row never showed %q", len(out), needle)
}
