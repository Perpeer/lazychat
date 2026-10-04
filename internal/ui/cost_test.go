package ui

import (
	"os"
	"testing"

	"lazychat/internal/ui/kit"
)

// TestFrameCost measures a whole frame and a tick with three running
// sessions, for the refactor's before-and-after numbers. It runs only when
// asked (LAZYCHAT_BENCH=1): a benchmark through the driver needs a test.
func TestFrameCost(t *testing.T) {
	if os.Getenv("LAZYCHAT_BENCH") == "" {
		t.Skip("LAZYCHAT_BENCH=1 runs it")
	}
	e, _ := seeded(t)
	d := start(t, e, 160, 48)
	for _, name := range []string{"ivy", "oak", "elm"} {
		d.key("n", "tab", "tab")
		d.typ(name)
		d.key("enter")
		d.expect("(ctrl+q) back to lazychat")
		d.leave()
	}
	view := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_ = d.app.View()
		}
	})
	n := 0
	tick := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			n++
			_, _ = d.app.Update(kit.Tick{N: n})
		}
	})
	t.Logf("view: %s %s", view, view.MemString())
	t.Logf("tick: %s %s", tick, tick.MemString())
}
