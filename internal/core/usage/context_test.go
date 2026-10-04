package usage

import (
	"path/filepath"
	"testing"
)

// The context is the newest call's; its base is the first call's, then the
// first after a compaction; what each tool's results fed it is measured by
// how the next call grew; each prompt's growth is its last call's context
// less the prompt before's.
func TestContextUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s-ctx.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the shed"}})
	w.reply("c1", 1000, 0, 50, use("b1", "Bash", map[string]any{"command": "ls"}), use("r1", "Read", map[string]any{}))
	w.results(nil, res("b1", 300), res("r1", 100))
	w.reply("c2", 400, 1050, 20)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "and the door"}})
	w.reply("c3", 600, 1471, 30)
	w.flush()
	r := Open(path)
	s, err := r.Update()
	if err != nil {
		t.Fatal(err)
	}
	c := s.Context()
	if c.Used != 2072 || c.Base != 1001 || c.Messages != 1071 || c.Window != 200_000 {
		t.Errorf("context %+v", c)
	}
	if s.Fed["Bash"] != 300 || s.Fed["Read"] != 100 {
		t.Errorf("fed %v", s.Fed)
	}
	if len(c.Growth) != 1 || c.Growth[0] != 2072-1451 {
		t.Errorf("growth %v", c.Growth)
	}
	if left := c.PromptsLeft(); left != int(c.Free()/(2072-1451)) {
		t.Errorf("prompts left %d", left)
	}

	w.add(map[string]any{"type": "user", "isCompactSummary": true, "message": map[string]any{"role": "user", "content": "the summary"}})
	w.reply("c4", 200, 0, 10)
	w.flush()
	if s, err = r.Update(); err != nil {
		t.Fatal(err)
	}
	c = s.Context()
	if c.Used != 201 || c.Base != 201 || c.Messages != 0 || len(s.Fed) != 0 {
		t.Errorf("after a compaction: %+v, fed %v", c, s.Fed)
	}
}

// A model's window and name are read from its id.
func TestModels(t *testing.T) {
	for _, c := range []struct {
		id, name string
		window   int64
	}{
		{"claude-opus-5-5", "Opus 5.5", 1_000_000},
		{"claude-sonnet-4-5-20250929", "Sonnet 4.5", 200_000},
		{"claude-haiku-4-5-20251001", "Haiku 4.5", 200_000},
		{"claude-fable-5-1", "Fable 5.1", 1_000_000},
		{"<synthetic>", "<synthetic>", 200_000},
	} {
		if got := ModelName(c.id); got != c.name {
			t.Errorf("ModelName(%q) = %q", c.id, got)
		}
		if got := ContextWindow(c.id); got != c.window {
			t.Errorf("ContextWindow(%q) = %d", c.id, got)
		}
	}
}
