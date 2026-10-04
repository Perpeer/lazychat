package kit

import (
	"strings"
	"testing"
	"time"

	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/term"
)

// The pane as a frame draws it: a 100×40 session full of coloured rows,
// boxed with its border, the cost every redraw pays.
func BenchmarkTermPaneView(b *testing.B) {
	zone.NewGlobal()
	script := `i=0; while [ $i -lt 60 ]; do printf '\033[32mrow %d\033[0m \033[1mgarden shed paints\033[0m blue door\n' $i; i=$((i+1)); done; sleep 30`
	s, err := term.Start(1, "bench", "p", b.TempDir(), []string{"/bin/sh", "-c", script}, 100, 40, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { term.StopAll([]*term.Session{s}, time.Second) })
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(s.Render(), "row 59") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	p := &TermPane{}
	p.Point("k", s, 100, 40)
	for b.Loop() {
		_ = Box("[2] session", p.View(100, 40, false, true), 102, 42, false, true)
	}
}
