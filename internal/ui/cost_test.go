package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/history"
	"lazychat/internal/core/state"
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

// writeBigTranscript lays an invented transcript at the scale of a long
// real session — prompts in the hundreds, calls and tool uses in the
// thousands — in the stand-in home, for timing the details page.
func writeBigTranscript(t *testing.T, e env, dir, id string, prompts, callsPer int) {
	t.Helper()
	folder := filepath.Join(e.history, history.Slug(dir))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Duration(prompts*callsPer+prompts) * 2 * time.Second).UTC()
	var b strings.Builder
	n := 0
	add := func(v map[string]any) {
		n++
		v["uuid"], v["timestamp"], v["cwd"], v["version"], v["gitBranch"] = fmt.Sprintf("%s-%d", id, n), at.Add(time.Duration(n)*time.Second).Format(time.RFC3339Nano), dir, "9.9.9", "blue-door"
		raw, _ := json.Marshal(v)
		b.Write(raw)
		b.WriteByte('\n')
	}
	for p := range prompts {
		add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": fmt.Sprintf("paint plank %d", p)}})
		for c := range callsPer {
			tool, input := "Read", map[string]any{"file_path": filepath.Join(dir, "paint", "door.go")}
			if c%2 == 1 {
				tool, input = "Bash", map[string]any{"command": "go test ./..."}
			}
			tid := fmt.Sprintf("t%d-%d", p, c)
			add(map[string]any{"type": "assistant", "message": map[string]any{"id": fmt.Sprintf("m%d-%d", p, c), "model": "claude-sonnet-4-5-20250929",
				"content": []any{map[string]any{"type": "tool_use", "id": tid, "name": tool, "input": input}},
				"usage":   map[string]any{"input_tokens": 2, "cache_creation_input_tokens": 400, "cache_read_input_tokens": n * 400, "output_tokens": 20}}})
			add(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": tid, "content": strings.Repeat("o", 300)}}}})
		}
		add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	}
	if err := os.WriteFile(filepath.Join(folder, id+".jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDetailsCost measures a frame of the details page on a session at
// the real scale: about 300 prompts, 3,000 calls and tool uses. It runs
// only when asked (LAZYCHAT_BENCH=1); the number is what a long session
// costs the loop on every redraw.
func TestDetailsCost(t *testing.T) {
	if os.Getenv("LAZYCHAT_BENCH") == "" {
		t.Skip("LAZYCHAT_BENCH=1 runs it")
	}
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "long work", ID: "garden-big"})
	e.lazyHome = t.TempDir()
	writeBigTranscript(t, e, dir, "garden-big", 300, 10)
	d := start(t, e, 180, 60)
	d.expect("long work")
	d.key("3")
	d.expect("prompts", "300 in all")
	view := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_ = d.app.View()
		}
	})
	t.Logf("details view: %s %s", view, view.MemString())
	d.quitApp()
}
