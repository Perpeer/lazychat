package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// bigSession writes an invented transcript at the scale of a long real
// one — prompts, calls, tool uses and subagents in the hundreds and
// thousands — so what the page derives from a session can be timed.
func bigSession(tb testing.TB, prompts, callsPer, agents int) *Session {
	tb.Helper()
	root := tb.TempDir()
	path := filepath.Join(root, "s-big.jsonl")
	w := newTranscript(tb, path)
	w.at = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	n := 0
	for p := range prompts {
		w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": fmt.Sprintf("paint plank %d", p)}})
		for c := range callsPer {
			n++
			id := fmt.Sprintf("c%d", n)
			tool := "Read"
			if c%2 == 1 {
				tool = "Bash"
			}
			w.reply(id, 400, int(n)*400, 20, use(id+"-t", tool, map[string]any{"file_path": "/garden/shed/door.go", "command": "go test"}))
			w.results(nil, res(id+"-t", 300))
		}
		if p%max(1, prompts/agents) == 0 {
			n++
			id := fmt.Sprintf("a%d", p)
			w.reply(fmt.Sprintf("c%d", n), 50, int(n)*400, 5, use(id, "Agent", map[string]any{"subagent_type": "Explore"}))
			w.results(map[string]any{"toolUseResult": map[string]any{"status": "completed", "agentId": id, "agentType": "Explore", "totalDurationMs": 20000}}, res(id, 40))
			sub := newTranscript(tb, filepath.Join(root, "s-big", "subagents", "agent-"+id+".jsonl"))
			sub.at = w.at
			for k := range 10 {
				sub.reply(fmt.Sprintf("%s-%d", id, k), 500, k*500, 30, use(fmt.Sprintf("%s-g%d", id, k), "Grep", map[string]any{}))
				sub.results(nil, res(fmt.Sprintf("%s-g%d", id, k), 60))
			}
			sub.flush()
		}
		w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	}
	w.flush()
	s, err := Open(path).Update()
	if err != nil {
		tb.Fatal(err)
	}
	return s
}

// BenchmarkDerive times what the details page derives from a session at
// the real scale on every read — and, before the fix, on every frame.
func BenchmarkDerive(b *testing.B) {
	s := bigSession(b, 300, 10, 30)
	b.Logf("%d prompts, %d calls, %d tool uses, %d agents", len(s.Prompts), len(s.Calls), len(s.ToolUses), len(s.Agents))
	b.Run("Turns", func(b *testing.B) {
		for b.Loop() {
			_ = s.Turns()
		}
	})
	b.Run("Context", func(b *testing.B) {
		for b.Loop() {
			_ = s.Context()
		}
	})
	b.Run("Timeline", func(b *testing.B) {
		for b.Loop() {
			_ = s.Timeline()
		}
	})
	b.Run("AllCalls", func(b *testing.B) {
		for b.Loop() {
			_ = s.AllCalls()
		}
	})
	b.Run("Clone", func(b *testing.B) {
		for b.Loop() {
			_ = s.Clone()
		}
	})
}

// TestReadBigFile times the first read of a real transcript named by
// LAZYCHAT_BENCH_FILE; nothing of the file enters the test's output but
// its size and times. Skipped without the variable.
func TestReadBigFile(t *testing.T) {
	path := os.Getenv("LAZYCHAT_BENCH_FILE")
	if path == "" {
		t.Skip("LAZYCHAT_BENCH_FILE names a transcript to time")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	r := Open(path)
	start := time.Now()
	s, err := r.Update()
	if err != nil {
		t.Fatal(err)
	}
	read := time.Since(start)
	start = time.Now()
	turns := s.Turns()
	tt := time.Since(start)
	start = time.Now()
	_ = s.Context()
	ct := time.Since(start)
	start = time.Now()
	_ = s.Clone()
	cl := time.Since(start)
	t.Logf("%d MB: read %v · %d prompts %d calls %d tool uses %d agents · Turns %v · Context %v · Clone %v", info.Size()>>20, read, len(turns), len(s.Calls), len(s.ToolUses), len(s.Agents), tt, ct, cl)
}
